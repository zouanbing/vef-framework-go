package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

// TestStartProcessor tests start processor scenarios.
func TestStartProcessor(t *testing.T) {
	p := NewStartProcessor()

	t.Run("NodeKind", func(t *testing.T) {
		assert.Equal(t, approval.NodeStart, p.NodeKind(), "Should return NodeStart kind")
	})

	t.Run("Process", func(t *testing.T) {
		result, err := p.Process(context.Background(), nil)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, NodeActionContinue, result.Action, "Should return Continue action")
	})
}

// TestEndProcessor tests end processor scenarios.
func TestEndProcessor(t *testing.T) {
	p := NewEndProcessor()

	t.Run("NodeKind", func(t *testing.T) {
		assert.Equal(t, approval.NodeEnd, p.NodeKind(), "Should return NodeEnd kind")
	})

	t.Run("Process", func(t *testing.T) {
		result, err := p.Process(context.Background(), nil)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, NodeActionComplete, result.Action, "Should return Complete action")
		assert.Equal(t, approval.InstanceApproved, result.FinalStatus, "Should set final status to Approved")
	})
}

// TestConditionProcessor tests condition processor scenarios.
func TestConditionProcessor(t *testing.T) {
	p := NewConditionProcessor()

	t.Run("NodeKind", func(t *testing.T) {
		assert.Equal(t, approval.NodeCondition, p.NodeKind(), "Should return NodeCondition kind")
	})

	t.Run("NoBranches", func(t *testing.T) {
		pc := &ProcessContext{
			Instance: &approval.Instance{ApplicantID: "u1"},
			Node:     &approval.FlowNode{},
		}
		_, err := p.Process(context.Background(), pc)
		require.Error(t, err, "Should return error when condition node has no branches")
		assert.Contains(t, err.Error(), "no branches", "Error should mention missing branches")
	})
}
