package cqrs

import (
	"context"

	icqrs "github.com/ilxqx/vef-framework-go/internal/cqrs"
)

type (
	Unit = icqrs.Unit
	Bus  = icqrs.Bus

	Handler[C any, R any]     = icqrs.Handler[C, R]
	HandlerFunc[C any, R any] = icqrs.HandlerFunc[C, R]

	Behavior     = icqrs.Behavior
	BehaviorFunc = icqrs.BehaviorFunc
)

var ErrHandlerNotFound = icqrs.ErrHandlerNotFound

// Register registers a type-safe handler for command type C.
// Panics if a handler is already registered for the same command type.
func Register[TCommand, TResult any](bus Bus, handler Handler[TCommand, TResult]) {
	icqrs.Register(bus, handler)
}

// Send dispatches a command through the behavior pipeline to its registered handler.
func Send[TCommand, TResult any](ctx context.Context, bus Bus, cmd TCommand) (TResult, error) {
	return icqrs.Send[TCommand, TResult](ctx, bus, cmd)
}
