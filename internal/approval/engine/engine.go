package engine

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

const maxNodeDepth = 100

type nodeDepthKey struct{}

// FlowEngine is the core engine for processing approval workflows.
type FlowEngine struct {
	registry   *strategy.StrategyRegistry
	processors map[approval.NodeKind]NodeProcessor
	publisher  *dispatcher.EventPublisher
}

// NewFlowEngine creates a new flow engine.
func NewFlowEngine(registry *strategy.StrategyRegistry, processors []NodeProcessor, pub *dispatcher.EventPublisher) *FlowEngine {
	engine := &FlowEngine{
		registry:   registry,
		processors: make(map[approval.NodeKind]NodeProcessor, len(processors)),
		publisher:  pub,
	}

	for _, p := range processors {
		engine.processors[p.NodeKind()] = p
	}

	return engine
}

// publishEvents publishes domain events if the publisher is available.
func (e *FlowEngine) publishEvents(ctx context.Context, db orm.DB, events ...approval.DomainEvent) error {
	if e.publisher == nil || len(events) == 0 {
		return nil
	}

	return e.publisher.PublishAll(ctx, db, events)
}

// StartProcess starts a flow process by finding the start node and processing it.
func (e *FlowEngine) StartProcess(ctx context.Context, db orm.DB, instance *approval.Instance) error {
	var startNode approval.FlowNode

	err := db.NewSelect().Model(&startNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", instance.FlowVersionID)
		c.Equals("kind", string(approval.NodeStart))
	}).Scan(ctx)
	if err != nil {
		return fmt.Errorf("find start node: %w", err)
	}

	return e.ProcessNode(ctx, db, instance, &startNode)
}

// ProcessNode dispatches a node to the appropriate processor.
func (e *FlowEngine) ProcessNode(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) error {
	depth, _ := ctx.Value(nodeDepthKey{}).(int)
	if depth >= maxNodeDepth {
		return fmt.Errorf("%w: depth=%d, node=%s", ErrMaxNodeDepth, depth, node.ID)
	}

	ctx = context.WithValue(ctx, nodeDepthKey{}, depth+1)

	processor, ok := e.processors[node.Kind]
	if !ok {
		return fmt.Errorf("%w: %s", ErrProcessorNotFound, node.Kind)
	}

	// Load assignee configs for the node
	var assignees []*approval.FlowNodeAssignee

	err := db.NewSelect().Model(&assignees).Where(func(c orm.ConditionBuilder) {
		c.Equals("node_id", node.ID)
	}).OrderBy("sort_order").Scan(ctx)
	if err != nil {
		return fmt.Errorf("load node assignees: %w", err)
	}

	pc := &ProcessContext{
		DB:          db,
		Instance:    instance,
		Node:        node,
		Assignees:   assignees,
		FormData:    approval.NewFormData(instance.FormData),
		ApplicantID: instance.ApplicantID,
		Registry:    e.registry,
	}

	result, err := processor.Process(ctx, pc)
	if err != nil {
		return err
	}

	return e.handleProcessResult(ctx, db, instance, node, result)
}

func (e *FlowEngine) handleProcessResult(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, result *ProcessResult) error {
	// Publish any events collected during processing
	if err := e.publishEvents(ctx, db, result.Events...); err != nil {
		return fmt.Errorf("publish processor events: %w", err)
	}

	switch result.Action {
	case NodeActionWait:
		instance.CurrentNodeID = new(node.ID)

		_, err := db.NewUpdate().Model(instance).WherePK().Exec(ctx)

		return err

	case NodeActionContinue:
		return e.AdvanceToNextNode(ctx, db, instance, node, result.BranchID)

	case NodeActionComplete:
		instance.CurrentNodeID = new(node.ID)
		instance.Status = result.FinalStatus
		instance.FinishedAt = new(timex.Now())

		if _, err := db.NewUpdate().Model(instance).WherePK().Exec(ctx); err != nil {
			return err
		}

		// Publish completion event
		if err := e.publishEvents(ctx, db,
			approval.NewInstanceCompletedEvent(instance.ID, result.FinalStatus),
		); err != nil {
			return fmt.Errorf("publish instance completed event: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unknown node action: %d", result.Action)
	}
}

// AdvanceToNextNode finds the matching edge from the current node and advances to the next one.
// branchID is used by condition nodes to select the edge matching the branch.
func (e *FlowEngine) AdvanceToNextNode(ctx context.Context, db orm.DB, instance *approval.Instance, fromNode *approval.FlowNode, branchID string) error {
	edge, err := e.findMatchingEdge(ctx, db, fromNode.ID, branchID)
	if err != nil {
		return err
	}

	var nextNode approval.FlowNode

	err = db.NewSelect().Model(&nextNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", edge.TargetNodeID)
	}).Scan(ctx)
	if err != nil {
		return fmt.Errorf("find next node: %w", err)
	}

	return e.ProcessNode(ctx, db, instance, &nextNode)
}

func (e *FlowEngine) findMatchingEdge(ctx context.Context, db orm.DB, sourceNodeID string, branchID string) (*approval.FlowEdge, error) {
	var edges []approval.FlowEdge

	err := db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("source_node_id", sourceNodeID)
		if branchID != "" {
			c.Equals("source_handle", branchID)
		}
	}).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find edges: %w", err)
	}

	if len(edges) == 0 {
		return nil, ErrNoMatchingEdge
	}

	return &edges[0], nil
}

// EvaluateNodeCompletion evaluates whether a node is complete based on its tasks and pass rule.
func (e *FlowEngine) EvaluateNodeCompletion(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) (approval.PassRuleResult, error) {
	var tasks []approval.Task

	err := db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("node_id", node.ID)
	}).Scan(ctx)
	if err != nil {
		return approval.PassRulePending, fmt.Errorf("query tasks: %w", err)
	}

	passStrategy, err := e.registry.GetPassRuleStrategy(node.PassRule)
	if err != nil {
		return approval.PassRulePending, err
	}

	prc := buildPassRuleContext(node, tasks)

	return passStrategy.Evaluate(prc), nil
}

// EvaluatePassRuleWithTasks evaluates the pass rule for a node using the provided tasks.
// This is used for simulation (e.g., checking if removing an assignee would deadlock the node).
func (e *FlowEngine) EvaluatePassRuleWithTasks(node *approval.FlowNode, tasks []approval.Task) (approval.PassRuleResult, error) {
	passStrategy, err := e.registry.GetPassRuleStrategy(node.PassRule)
	if err != nil {
		return approval.PassRulePending, err
	}

	prc := buildPassRuleContext(node, tasks)

	return passStrategy.Evaluate(prc), nil
}

func buildPassRuleContext(node *approval.FlowNode, tasks []approval.Task) approval.PassRuleContext {
	ctx := approval.PassRuleContext{
		PassRatio: NormalizePassRatio(node.PassRatio.InexactFloat64()),
	}

	for _, t := range tasks {
		// Exclude non-actionable tasks from total count:
		// transferred, canceled, removed, skipped are no longer participating
		switch t.Status {
		case approval.TaskTransferred, approval.TaskCanceled, approval.TaskRemoved, approval.TaskSkipped:
			continue
		}

		ctx.TotalCount++

		switch t.Status {
		case approval.TaskApproved, approval.TaskHandled:
			ctx.ApprovedCount++
		case approval.TaskRejected:
			ctx.RejectedCount++
		}
	}

	return ctx
}

// NormalizePassRatio normalizes pass ratio to 0-100 scale.
// Values in (0, 1] range are treated as proportions and converted to percentage.
// E.g., 0.6 → 60, 1.0 → 100. Values > 1 are kept as-is (already percentage).
// Negative values are clamped to 0, values above 100 are clamped to 100.
func NormalizePassRatio(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}

	if ratio <= 1 {
		return ratio * 100
	}

	if ratio > 100 {
		return 100
	}

	return ratio
}

// PredictNextNode predicts the next node and its assignees.
func (e *FlowEngine) PredictNextNode(ctx context.Context, db orm.DB, instance *approval.Instance, fromNode *approval.FlowNode) (*approval.FlowNode, []string, error) {
	edge, err := e.findMatchingEdge(ctx, db, fromNode.ID, "")
	if err != nil {
		return nil, nil, err
	}

	var nextNode approval.FlowNode

	err = db.NewSelect().Model(&nextNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", edge.TargetNodeID)
	}).Scan(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("find next node: %w", err)
	}

	assigneeIDs := e.predictAssignees(ctx, db, instance, &nextNode)

	return &nextNode, assigneeIDs, nil
}

func (e *FlowEngine) predictAssignees(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) []string {
	processor, ok := e.processors[node.Kind]
	if !ok {
		return nil
	}

	predictor, ok := processor.(NodePredictor)
	if !ok {
		return nil
	}

	var assignees []*approval.FlowNodeAssignee
	_ = db.NewSelect().Model(&assignees).Where(func(c orm.ConditionBuilder) {
		c.Equals("node_id", node.ID)
	}).OrderBy("sort_order").Scan(ctx)

	pc := &ProcessContext{
		DB:          db,
		Instance:    instance,
		Node:        node,
		Assignees:   assignees,
		FormData:    approval.NewFormData(instance.FormData),
		ApplicantID: instance.ApplicantID,
		Registry:    e.registry,
	}

	ids, err := predictor.Predict(ctx, pc)
	if err != nil {
		return nil
	}

	return ids
}
