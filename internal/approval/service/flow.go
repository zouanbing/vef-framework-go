package service

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/ilxqx/go-collections"
	"github.com/ilxqx/vef-framework-go/approval"
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
			data, err := nd.ParseData()
			if err != nil {
				return fmt.Errorf("parse sub_flow node %q data: %w", nd.ID, err)
			}

			sfData := data.(*approval.SubFlowNodeData)
			if sfData.SubFlowConfig == nil || sfData.SubFlowConfig.FlowID == "" {
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
