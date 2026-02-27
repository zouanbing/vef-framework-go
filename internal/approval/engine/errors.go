package engine

import "errors"

var (
	// Engine errors
	ErrNoMatchingEdge    = errors.New("no matching outgoing edge for node")
	ErrProcessorNotFound = errors.New("node processor not found for node kind")
	ErrMaxNodeDepth      = errors.New("max node processing depth exceeded")

	// Approval node errors
	ErrNoAssignee = errors.New("no assignee resolved for node")

	// Condition node errors
	ErrNoBranches       = errors.New("condition node has no branches")
	ErrNoMatchingBranch = errors.New("no matching branch and no default branch")
)
