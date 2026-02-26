package cqrs

import "context"

// Unit is a placeholder return type for commands that produce no result.
type Unit struct{}

// Handler is a type-safe command/query handler.
type Handler[C any, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc[C any, R any] func(ctx context.Context, cmd C) (R, error)

func (f HandlerFunc[C, R]) Handle(ctx context.Context, cmd C) (R, error) {
	return f(ctx, cmd)
}

// Behavior is a Bus middleware that can intercept all commands/queries.
type Behavior interface {
	Handle(ctx context.Context, cmd any, next func(ctx context.Context) (any, error)) (any, error)
}

// BehaviorFunc is a function adapter for Behavior.
type BehaviorFunc func(ctx context.Context, cmd any, next func(ctx context.Context) (any, error)) (any, error)

func (f BehaviorFunc) Handle(ctx context.Context, cmd any, next func(ctx context.Context) (any, error)) (any, error) {
	return f(ctx, cmd, next)
}
