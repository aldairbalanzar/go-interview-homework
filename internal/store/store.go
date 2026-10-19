// Package store reads and writes todo data in Postgres.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Store struct {
	DB *sql.DB
}

type User struct {
	ID    int64
	Email string
	Name  string
}

type Task struct {
	ID          int64
	UserID      int64
	Title       string
	Description *string
	Status      string
	DueDate     *string
	Tags        []string
}

func (s *Store) User(ctx context.Context, id int64) (*User, error) {
	var u User
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, email, name FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", id, err)
	}
	return &u, nil
}

func (s *Store) TaskCount(ctx context.Context, userID int64, status *string) (int, error) {
	q := `SELECT COUNT(*) FROM tasks WHERE user_id = $1`
	args := []any{userID}
	if status != nil {
		q += ` AND status = $2`
		args = append(args, *status)
	}
	var n int
	if err := s.DB.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("task count for user %d: %w", userID, err)
	}
	return n, nil
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, email, name FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) Task(ctx context.Context, id int64) (*Task, error) {
	t, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task %d: %w", id, err)
	}
	if err := s.attachTags(ctx, []*Task{t}); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) Tasks(ctx context.Context, userID *int64, status *string) ([]*Task, error) {
	q := taskSelect + ` WHERE 1=1`
	args := []any{}
	n := 1
	if userID != nil {
		q += fmt.Sprintf(` AND user_id = $%d`, n)
		args = append(args, *userID)
		n++
	}
	if status != nil {
		q += fmt.Sprintf(` AND status = $%d`, n)
		args = append(args, *status)
		n++
	}
	q += ` ORDER BY id`

	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachTags(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CreateTask(ctx context.Context, userID int64, title string, description *string, dueDate *string, status string, tags []string) (*Task, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var exists int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1`, userID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user %d not found", userID)
	}
	if err != nil {
		return nil, err
	}

	var due any
	if dueDate != nil && *dueDate != "" {
		if _, err := time.Parse("2006-01-02", *dueDate); err != nil {
			return nil, fmt.Errorf("dueDate must be YYYY-MM-DD")
		}
		due = *dueDate
	}

	var desc any
	if description != nil && *description != "" {
		desc = *description
	}

	var t Task
	t.UserID = userID
	t.Title = title
	t.Description = description
	t.Status = status
	t.DueDate = dueDate
	t.Tags = tags
	if t.Tags == nil {
		t.Tags = []string{}
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO tasks (user_id, title, description, status, due_date)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID, title, desc, status, due,
	).Scan(&t.ID)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_tags (task_id, tag) VALUES ($1, $2)`,
			t.ID, tag,
		); err != nil {
			return nil, fmt.Errorf("insert tag: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) UpdateTaskStatus(ctx context.Context, id int64, status string) (*Task, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE tasks SET status = $1, updated_at = now() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update task %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("task %d not found", id)
	}
	return s.Task(ctx, id)
}

const taskSelect = `SELECT id, user_id, title, description, status::text, due_date FROM tasks`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (*Task, error) {
	var t Task
	var desc sql.NullString
	var due sql.NullTime
	if err := row.Scan(&t.ID, &t.UserID, &t.Title, &desc, &t.Status, &due); err != nil {
		return nil, err
	}
	if desc.Valid {
		t.Description = &desc.String
	}
	if due.Valid {
		s := due.Time.Format("2006-01-02")
		t.DueDate = &s
	}
	t.Tags = []string{}
	return &t, nil
}

func (s *Store) attachTags(ctx context.Context, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]any, len(tasks))
	ph := make([]string, len(tasks))
	byID := make(map[int64]*Task, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
		ph[i] = "$" + strconv.Itoa(i+1)
		byID[t.ID] = t
		t.Tags = []string{}
	}
	q := `SELECT task_id, tag FROM task_tags WHERE task_id IN (` + strings.Join(ph, ",") + `) ORDER BY tag`
	rows, err := s.DB.QueryContext(ctx, q, ids...)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var taskID int64
		var tag string
		if err := rows.Scan(&taskID, &tag); err != nil {
			return err
		}
		if t := byID[taskID]; t != nil {
			t.Tags = append(t.Tags, tag)
		}
	}
	return rows.Err()
}
