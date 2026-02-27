package cqrs

import (
	"context"
	"reflect"
)

// Bus is the command/query dispatch bus interface.
type Bus interface {
	register(key reflect.Type, d Dispatcher)
	send(ctx context.Context, key reflect.Type, action Action) (any, error)
}

// Action is the base interface for all commands and queries.
type Action interface {
	Kind() ActionKind
}

// Handler is a type-safe command/query handler.
type Handler[TAction Action, TResult any] interface {
	Handle(ctx context.Context, action TAction) (TResult, error)
}

// Behavior is a Bus middleware that can intercept all commands/queries.
type Behavior interface {
	Handle(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error)
}
