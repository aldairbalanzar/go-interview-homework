package graph

import (
	"context"

	"github.com/example/todo-homework/graph/model"
	"github.com/example/todo-homework/internal/store"
)

// Resolver is the GraphQL root. Store is the Postgres data layer.
type Resolver struct {
	Store *store.Store
}

func (r *Resolver) taskWithUser(ctx context.Context, t *store.Task) (*model.Task, error) {
	u, err := r.Store.User(ctx, t.UserID)
	if err != nil {
		return nil, err
	}
	return toGQLTask(t, toGQLUser(u)), nil
}
