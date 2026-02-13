package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

func TestInstanceStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from approval.InstanceStatus
		to   approval.InstanceStatus
	}{
		{"RunningToApproved", approval.InstanceRunning, approval.InstanceApproved},
		{"RunningToRejected", approval.InstanceRunning, approval.InstanceRejected},
		{"RunningToWithdrawn", approval.InstanceRunning, approval.InstanceWithdrawn},
		{"RunningToReturned", approval.InstanceRunning, approval.InstanceReturned},
		{"ReturnedToRunning", approval.InstanceReturned, approval.InstanceRunning},
		{"WithdrawnToRunning", approval.InstanceWithdrawn, approval.InstanceRunning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, InstanceStateMachine.CanTransition(tt.from, tt.to))
			assert.NoError(t, InstanceStateMachine.Transition(tt.from, tt.to))
		})
	}
}

func TestInstanceStateMachine_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from approval.InstanceStatus
		to   approval.InstanceStatus
	}{
		{"ApprovedToRunning", approval.InstanceApproved, approval.InstanceRunning},
		{"RejectedToApproved", approval.InstanceRejected, approval.InstanceApproved},
		{"ApprovedToRejected", approval.InstanceApproved, approval.InstanceRejected},
		{"RejectedToRunning", approval.InstanceRejected, approval.InstanceRunning},
		{"ReturnedToApproved", approval.InstanceReturned, approval.InstanceApproved},
		{"RunningToRunning", approval.InstanceRunning, approval.InstanceRunning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, InstanceStateMachine.CanTransition(tt.from, tt.to))
			assert.Error(t, InstanceStateMachine.Transition(tt.from, tt.to))
		})
	}
}

func TestTaskStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from approval.TaskStatus
		to   approval.TaskStatus
	}{
		{"WaitingToPending", approval.TaskWaiting, approval.TaskPending},
		{"WaitingToCanceled", approval.TaskWaiting, approval.TaskCanceled},
		{"WaitingToSkipped", approval.TaskWaiting, approval.TaskSkipped},
		{"PendingToApproved", approval.TaskPending, approval.TaskApproved},
		{"PendingToHandled", approval.TaskPending, approval.TaskHandled},
		{"PendingToRejected", approval.TaskPending, approval.TaskRejected},
		{"PendingToTransferred", approval.TaskPending, approval.TaskTransferred},
		{"PendingToRollback", approval.TaskPending, approval.TaskRollback},
		{"PendingToCanceled", approval.TaskPending, approval.TaskCanceled},
		{"PendingToWaiting", approval.TaskPending, approval.TaskWaiting},
		{"PendingToRemoved", approval.TaskPending, approval.TaskRemoved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, TaskStateMachine.CanTransition(tt.from, tt.to))
			assert.NoError(t, TaskStateMachine.Transition(tt.from, tt.to))
		})
	}
}

func TestTaskStateMachine_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from approval.TaskStatus
		to   approval.TaskStatus
	}{
		{"ApprovedToPending", approval.TaskApproved, approval.TaskPending},
		{"RejectedToApproved", approval.TaskRejected, approval.TaskApproved},
		{"CanceledToPending", approval.TaskCanceled, approval.TaskPending},
		{"TransferredToPending", approval.TaskTransferred, approval.TaskPending},
		{"RemovedToPending", approval.TaskRemoved, approval.TaskPending},
		{"SkippedToPending", approval.TaskSkipped, approval.TaskPending},
		{"WaitingToApproved", approval.TaskWaiting, approval.TaskApproved},
		{"WaitingToRejected", approval.TaskWaiting, approval.TaskRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, TaskStateMachine.CanTransition(tt.from, tt.to))
			assert.Error(t, TaskStateMachine.Transition(tt.from, tt.to))
		})
	}
}

func TestInstanceStateMachine_AvailableTransitions(t *testing.T) {
	targets := InstanceStateMachine.AvailableTransitions(approval.InstanceRunning)
	require.Len(t, targets, 4, "Should have 4 available transitions from running")

	targetSet := make(map[approval.InstanceStatus]bool)
	for _, s := range targets {
		targetSet[s] = true
	}

	assert.True(t, targetSet[approval.InstanceApproved], "Should include approved")
	assert.True(t, targetSet[approval.InstanceRejected], "Should include rejected")
	assert.True(t, targetSet[approval.InstanceWithdrawn], "Should include withdrawn")
	assert.True(t, targetSet[approval.InstanceReturned], "Should include returned")
}

func TestTaskStateMachine_AvailableTransitions(t *testing.T) {
	pending := TaskStateMachine.AvailableTransitions(approval.TaskPending)
	assert.Len(t, pending, 8, "Should have 8 available transitions from pending")

	waiting := TaskStateMachine.AvailableTransitions(approval.TaskWaiting)
	assert.Len(t, waiting, 3, "Should have 3 available transitions from waiting")

	targetSet := make(map[approval.TaskStatus]bool)
	for _, state := range pending {
		targetSet[state] = true
	}
	assert.True(t, targetSet[approval.TaskHandled], "Should include handled in pending transitions")

	// Final states have no transitions
	approved := TaskStateMachine.AvailableTransitions(approval.TaskApproved)
	assert.Empty(t, approved, "Should have no transitions from final state")
}

func TestStateMachine_TransitionError(t *testing.T) {
	err := InstanceStateMachine.Transition(approval.InstanceApproved, approval.InstanceRunning)
	require.Error(t, err, "Should return error for invalid transition")
	assert.Contains(t, err.Error(), "invalid instance transition")
	assert.Contains(t, err.Error(), "approved")
	assert.Contains(t, err.Error(), "running")
}
