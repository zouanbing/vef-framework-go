package shared

import "github.com/ilxqx/vef-framework-go/approval"

// CreateFlowInitiatorCmd contains the parameters for creating a flow initiator.
type CreateFlowInitiatorCmd struct {
	Kind approval.InitiatorKind
	IDs  []string
}
