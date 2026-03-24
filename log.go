package vef

import (
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// NamedLogger creates a named logger instance for the specified component.
// This is a convenience function that wraps the internal logger factory.
func NamedLogger(name string) logx.Logger {
	return ilogx.Named(name)
}
