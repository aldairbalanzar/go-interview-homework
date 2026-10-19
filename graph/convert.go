package graph

import (
	"strconv"

	"github.com/example/todo-homework/graph/model"
	"github.com/example/todo-homework/internal/store"
)

func parseID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

func statusToDB(s model.TaskStatus) string {
	switch s {
	case model.TaskStatusInProgress:
		return "in_progress"
	case model.TaskStatusDone:
		return "done"
	default:
		return "pending"
	}
}

func statusToGQL(s string) model.TaskStatus {
	switch s {
	case "in_progress":
		return model.TaskStatusInProgress
	case "done":
		return model.TaskStatusDone
	default:
		return model.TaskStatusPending
	}
}

func toGQLUser(u *store.User) *model.User {
	if u == nil {
		return nil
	}
	return &model.User{
		ID:    strconv.FormatInt(u.ID, 10),
		Email: u.Email,
		Name:  u.Name,
	}
}

func toGQLTask(t *store.Task, user *model.User) *model.Task {
	if t == nil {
		return nil
	}
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	return &model.Task{
		ID:          strconv.FormatInt(t.ID, 10),
		User:        user,
		Title:       t.Title,
		Description: t.Description,
		Status:      statusToGQL(t.Status),
		DueDate:     t.DueDate,
		Tags:        tags,
	}
}
