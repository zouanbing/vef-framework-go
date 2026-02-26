package cqrs

import (
	"context"
	"reflect"
)

// Bus is the command/query dispatch bus interface.
// Unexported methods prevent external implementation.
type Bus interface {
	register(key reflect.Type, d Dispatcher)
	send(ctx context.Context, key reflect.Type, cmd any) (any, error)
}
