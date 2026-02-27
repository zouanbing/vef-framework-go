package cqrs

import "context"

// ActionKind distinguishes commands from queries.
type ActionKind int

const (
	Command ActionKind = iota
	Query
)

// Action is the base interface for all commands and queries.
type Action interface {
	Kind() ActionKind
}

// CommandBase is embedded by command types to mark them as commands.
type CommandBase struct{}

func (CommandBase) Kind() ActionKind { return Command }

// QueryBase is embedded by query types to mark them as queries.
type QueryBase struct{}

func (QueryBase) Kind() ActionKind { return Query }

// Unit is a placeholder return type for commands that produce no result.
type Unit struct{}

// Handler is a type-safe command/query handler.
type Handler[C Action, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc[C Action, R any] func(ctx context.Context, cmd C) (R, error)

func (f HandlerFunc[C, R]) Handle(ctx context.Context, cmd C) (R, error) {
	return f(ctx, cmd)
}

// Behavior is a Bus middleware that can intercept all commands/queries.
type Behavior interface {
	Handle(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error)
}

// BehaviorFunc is a function adapter for Behavior.
type BehaviorFunc func(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error)

func (f BehaviorFunc) Handle(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error) {
	return f(ctx, action, next)
}
