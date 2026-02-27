package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/ilxqx/go-collections"
	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/decimal"
)

// validNodeKinds defines the set of valid node kinds for flow validation.
var validNodeKinds = collections.NewHashSetFrom(
	approval.NodeStart,
	approval.NodeEnd,
	approval.NodeApproval,
	approval.NodeHandle,
	approval.NodeCondition,
	approval.NodeCC,
	approval.NodeSubFlow,
)

// FlowService provides flow-level domain operations.
type FlowService struct{}

// NewFlowService creates a new FlowService.
func NewFlowService() *FlowService {
	return &FlowService{}
}

// ValidateFlowDefinition validates the structural integrity of a flow definition.
func (s *FlowService) ValidateFlowDefinition(def *approval.FlowDefinition) error {
	if len(def.Nodes) == 0 {
		return fmt.Errorf("flow must have at least one node")
	}

	var startCount, endCount int

	nodeIDs := make(map[string]struct{}, len(def.Nodes))

	for _, nd := range def.Nodes {
		if nd.ID == "" {
			return fmt.Errorf("node ID must not be empty")
		}

		if _, dup := nodeIDs[nd.ID]; dup {
			return fmt.Errorf("duplicate node ID %q", nd.ID)
		}

		nodeIDs[nd.ID] = struct{}{}

		if !validNodeKinds.Contains(nd.Type) {
			return fmt.Errorf("invalid node kind %q for node %q", nd.Type, nd.ID)
		}

		switch nd.Type {
		case approval.NodeStart:
			startCount++
		case approval.NodeEnd:
			endCount++
		case approval.NodeSubFlow:
			var config map[string]any
			if nd.Data != nil {
				config, _ = nd.Data["subFlowConfig"].(map[string]any)
			}

			if config == nil {
				return fmt.Errorf("sub_flow node %q missing subFlowConfig", nd.ID)
			}

			flowID, _ := config["flowId"].(string)
			if flowID == "" {
				return fmt.Errorf("sub_flow node %q missing flowId in subFlowConfig", nd.ID)
			}
		}
	}

	if startCount != 1 {
		return fmt.Errorf("flow must have exactly 1 start node, found %d", startCount)
	}

	if endCount < 1 {
		return fmt.Errorf("flow must have at least 1 end node, found %d", endCount)
	}

	for _, edge := range def.Edges {
		if _, ok := nodeIDs[edge.Source]; !ok {
			return fmt.Errorf("line references unknown source node %q", edge.Source)
		}

		if _, ok := nodeIDs[edge.Target]; !ok {
			return fmt.Errorf("line references unknown target node %q", edge.Target)
		}
	}

	return nil
}

// ApplyNodeData maps design-time data to FlowNode fields.
func (s *FlowService) ApplyNodeData(node *approval.FlowNode, data map[string]any) {
	if len(data) == 0 {
		return
	}

	if v, ok := data["description"].(string); ok {
		node.Description = &v
	}

	if v, ok := data["isReadConfirmRequired"].(bool); ok {
		node.IsReadConfirmRequired = v
	}

	if v, ok := data["approvalMethod"].(string); ok {
		node.ApprovalMethod = approval.ApprovalMethod(v)
	}

	if v, ok := data["passRule"].(string); ok {
		node.PassRule = approval.PassRule(v)
	}

	if v, ok := data["executionType"].(string); ok {
		node.ExecutionType = approval.ExecutionType(v)
	}

	if v, ok := data["emptyHandlerAction"].(string); ok {
		node.EmptyHandlerAction = approval.EmptyHandlerAction(v)
	}

	if v, ok := data["sameApplicantAction"].(string); ok {
		node.SameApplicantAction = approval.SameApplicantAction(v)
	}

	if v, ok := data["duplicateHandlerAction"].(string); ok {
		node.DuplicateHandlerAction = approval.DuplicateHandlerAction(v)
	}

	if v, ok := data["rollbackType"].(string); ok {
		node.RollbackType = approval.RollbackType(v)
	}

	if v, ok := data["rollbackDataStrategy"].(string); ok {
		node.RollbackDataStrategy = approval.RollbackDataStrategy(v)
	}

	if v, ok := data["isRollbackAllowed"].(bool); ok {
		node.IsRollbackAllowed = v
	}

	if v, ok := data["isAddAssigneeAllowed"].(bool); ok {
		node.IsAddAssigneeAllowed = v
	}

	if v, ok := data["isRemoveAssigneeAllowed"].(bool); ok {
		node.IsRemoveAssigneeAllowed = v
	}

	if v, ok := data["isTransferAllowed"].(bool); ok {
		node.IsTransferAllowed = v
	}

	if v, ok := data["isOpinionRequired"].(bool); ok {
		node.IsOpinionRequired = v
	}

	if v, ok := data["isManualCcAllowed"].(bool); ok {
		node.IsManualCCAllowed = v
	}

	if v, ok := data["passRatio"]; ok {
		switch r := v.(type) {
		case float64:
			node.PassRatio = decimal.NewFromFloat(r)
		case string:
			if d, err := decimal.NewFromString(r); err == nil {
				node.PassRatio = d
			}
		}
	}

	if v, ok := data["timeoutHours"]; ok {
		if f, ok := v.(float64); ok {
			node.TimeoutHours = int(f)
		}
	}

	if v, ok := data["timeoutAction"].(string); ok {
		node.TimeoutAction = approval.TimeoutAction(v)
	}

	if v, ok := data["timeoutNotifyBeforeHours"]; ok {
		if f, ok := v.(float64); ok {
			node.TimeoutNotifyBeforeHours = int(f)
		}
	}

	if v, ok := data["urgeCooldownMinutes"]; ok {
		if f, ok := v.(float64); ok {
			node.UrgeCooldownMinutes = int(f)
		}
	}

	if v, ok := data["adminUserIds"]; ok {
		if ids, ok := ToStringSlice(v); ok {
			node.AdminUserIDs = ids
		}
	}

	if v, ok := data["fallbackUserIds"]; ok {
		if ids, ok := ToStringSlice(v); ok {
			node.FallbackUserIDs = ids
		}
	}

	if v, ok := data["addAssigneeTypes"]; ok {
		if ids, ok := ToStringSlice(v); ok {
			node.AddAssigneeTypes = ids
		}
	}

	if v, ok := data["subFlowConfig"].(map[string]any); ok {
		node.SubFlowConfig = v
	}

	if v, ok := data["fieldPermissions"].(map[string]any); ok {
		perms := make(map[string]approval.Permission, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				perms[k] = approval.Permission(s)
			}
		}

		node.FieldPermissions = perms
	}

	branches := ExtractFromData[approval.ConditionBranch](data, "branches")
	if len(branches) > 0 {
		node.Branches = branches
	}
}

// ExtractFromData extracts a typed slice from a map key via JSON round-trip.
func ExtractFromData[T any](data map[string]any, key string) []T {
	if data == nil {
		return nil
	}

	raw, ok := data[key]
	if !ok {
		return nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var result []T
	if err := json.Unmarshal(b, &result); err != nil {
		return nil
	}

	return result
}

// ToStringSlice converts a JSON-decoded []any to []string.
func ToStringSlice(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}

	result := make([]string, 0, len(arr))

	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}

	return result, len(result) > 0
}

// BuildTitleTemplateData builds a camelCase map for rendering instance title templates.
func (s *FlowService) BuildTitleTemplateData(flowName, flowCode, instanceNo string, formData map[string]any) map[string]any {
	return map[string]any{
		"flowName":   flowName,
		"flowCode":   flowCode,
		"instanceNo": instanceNo,
		"formData":   formData,
	}
}

// RenderTitle renders an instance title from a Go text/template string.
func (s *FlowService) RenderTitle(titleTemplate string, data map[string]any) (string, error) {
	if titleTemplate == "" {
		return data["flowName"].(string) + "-" + data["instanceNo"].(string), nil
	}

	tmpl, err := template.New("title").Parse(titleTemplate)
	if err != nil {
		return "", fmt.Errorf("parse title template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute title template: %w", err)
	}

	return buf.String(), nil
}
