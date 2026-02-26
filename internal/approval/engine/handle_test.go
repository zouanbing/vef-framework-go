package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

// TestNewHandleProcessor tests new handle processor scenarios.
func TestNewHandleProcessor(t *testing.T) {
	p := NewHandleProcessor(nil)
	require.NotNil(t, p, "Should return a non-nil processor")
	assert.Equal(t, approval.NodeHandle, p.NodeKind(), "Should return NodeHandle kind")
}
