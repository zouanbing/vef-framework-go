package cqrs

import (
	"context"

	icqrs "github.com/ilxqx/vef-framework-go/internal/cqrs"
)

type (
	ActionKind = icqrs.ActionKind
	Action     = icqrs.Action

	CommandBase = icqrs.CommandBase
	QueryBase   = icqrs.QueryBase

	Unit = icqrs.Unit
	Bus  = icqrs.Bus

	Handler[C icqrs.Action, R any]     = icqrs.Handler[C, R]
	HandlerFunc[C icqrs.Action, R any] = icqrs.HandlerFunc[C, R]

	Behavior     = icqrs.Behavior
	BehaviorFunc = icqrs.BehaviorFunc
)

const (
	Command = icqrs.Command
	Query   = icqrs.Query
)

var ErrHandlerNotFound = icqrs.ErrHandlerNotFound

// Register registers a type-safe handler for command type C.
// Panics if a handler is already registered for the same command type.
func Register[TCommand icqrs.Action, TResult any](bus Bus, handler Handler[TCommand, TResult]) {
	icqrs.Register(bus, handler)
}

// Send dispatches a command through the behavior pipeline to its registered handler.
func Send[TCommand icqrs.Action, TResult any](ctx context.Context, bus Bus, cmd TCommand) (TResult, error) {
	return icqrs.Send[TCommand, TResult](ctx, bus, cmd)
}
