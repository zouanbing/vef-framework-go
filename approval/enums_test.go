package approval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddAssigneeType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		value    AddAssigneeType
		expected bool
	}{
		{"Before", AddAssigneeBefore, true},
		{"After", AddAssigneeAfter, true},
		{"Parallel", AddAssigneeParallel, true},
		{"InvalidEmpty", AddAssigneeType(""), false},
		{"InvalidRandom", AddAssigneeType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.value.IsValid())
		})
	}
}

func TestInstanceStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   InstanceStatus
		expected string
	}{
		{"Running", InstanceRunning, "running"},
		{"Approved", InstanceApproved, "approved"},
		{"Rejected", InstanceRejected, "rejected"},
		{"Withdrawn", InstanceWithdrawn, "withdrawn"},
		{"Returned", InstanceReturned, "returned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestInstanceStatus_IsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   InstanceStatus
		expected bool
	}{
		{"Approved", InstanceApproved, true},
		{"Rejected", InstanceRejected, true},
		{"Withdrawn", InstanceWithdrawn, true},
		{"Running", InstanceRunning, false},
		{"Returned", InstanceReturned, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsFinal())
		})
	}
}

func TestTaskStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   TaskStatus
		expected string
	}{
		{"Waiting", TaskWaiting, "waiting"},
		{"Pending", TaskPending, "pending"},
		{"Approved", TaskApproved, "approved"},
		{"Rejected", TaskRejected, "rejected"},
		{"Handled", TaskHandled, "handled"},
		{"Transferred", TaskTransferred, "transferred"},
		{"Rollback", TaskRollback, "rollback"},
		{"Canceled", TaskCanceled, "canceled"},
		{"Removed", TaskRemoved, "removed"},
		{"Skipped", TaskSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestTaskStatus_IsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   TaskStatus
		expected bool
	}{
		{"Approved", TaskApproved, true},
		{"Rejected", TaskRejected, true},
		{"Handled", TaskHandled, true},
		{"Transferred", TaskTransferred, true},
		{"Rollback", TaskRollback, true},
		{"Canceled", TaskCanceled, true},
		{"Removed", TaskRemoved, true},
		{"Skipped", TaskSkipped, true},
		{"Waiting", TaskWaiting, false},
		{"Pending", TaskPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsFinal())
		})
	}
}
