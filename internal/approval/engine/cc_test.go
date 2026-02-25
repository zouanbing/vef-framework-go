package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

func TestNewCCProcessor(t *testing.T) {
	p := NewCCProcessor()
	require.NotNil(t, p, "Should return a non-nil processor")
	assert.Equal(t, approval.NodeCC, p.NodeKind(), "Should return NodeCC kind")
}

func TestCCProcessorContinueWhenNotBlocking(t *testing.T) {
	p := NewCCProcessor()
	pc := &ProcessContext{
		Node: &approval.FlowNode{
			IsReadConfirmRequired: false,
		},
		Instance: &approval.Instance{},
	}
	result, err := p.Process(t.Context(), pc)
	require.NoError(t, err, "Should not return error")
	assert.Equal(t, NodeActionContinue, result.Action, "Should continue when not blocking")
}

func TestCCProcessorWaitWhenBlocking(t *testing.T) {
	p := NewCCProcessor()
	pc := &ProcessContext{
		Node: &approval.FlowNode{
			IsReadConfirmRequired: true,
		},
		Instance: &approval.Instance{},
	}
	result, err := p.Process(t.Context(), pc)
	require.NoError(t, err, "Should not return error")
	assert.Equal(t, NodeActionWait, result.Action, "Should wait when blocking")
}
