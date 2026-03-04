package service

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/orm"
)

// ValidationService provides validation operations.
type ValidationService struct {
	assigneeService approval.AssigneeService
}

// NewValidationService creates a new ValidationService.
func NewValidationService(assigneeSvc approval.AssigneeService) *ValidationService {
	return &ValidationService{assigneeService: assigneeSvc}
}

// ValidateOpinion checks if an opinion is required but missing.
func (s *ValidationService) ValidateOpinion(node *approval.FlowNode, opinion string) error {
	if node.IsOpinionRequired && opinion == "" {
		return shared.ErrOpinionRequired
	}

	return nil
}

// ValidateRollbackTarget validates the rollback target node based on the node's RollbackType.
func (s *ValidationService) ValidateRollbackTarget(ctx context.Context, db orm.DB, instance *approval.Instance, currentNode *approval.FlowNode, targetNodeID string) error {
	switch currentNode.RollbackType {
	case approval.RollbackNone:
		return shared.ErrRollbackNotAllowed

	case approval.RollbackPrevious:
		count, err := db.NewSelect().
			Model((*approval.FlowEdge)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("source_node_id", targetNodeID).
					Equals("target_node_id", currentNode.ID).
					Equals("flow_version_id", instance.FlowVersionID)
			}).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("find previous node: %w", err)
		}

		if count == 0 {
			return shared.ErrInvalidRollbackTarget
		}

	case approval.RollbackStart:
		var startNode approval.FlowNode

		if err := db.NewSelect().
			Model(&startNode).
			Select("id").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("flow_version_id", instance.FlowVersionID).
					Equals("kind", string(approval.NodeStart))
			}).
			Scan(ctx); err != nil {
			return fmt.Errorf("find start node: %w", err)
		}

		if startNode.ID != targetNodeID {
			return shared.ErrInvalidRollbackTarget
		}

	case approval.RollbackAny:
		count, err := db.NewSelect().
			Model((*approval.FlowNode)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", targetNodeID).
					Equals("flow_version_id", instance.FlowVersionID)
			}).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("find rollback target node: %w", err)
		}

		if count == 0 {
			return shared.ErrInvalidRollbackTarget
		}

	case approval.RollbackSpecified:
		var targetNode approval.FlowNode

		if err := db.NewSelect().
			Model(&targetNode).
			Select("key").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", targetNodeID).
					Equals("flow_version_id", instance.FlowVersionID)
			}).
			Scan(ctx); err != nil {
			return shared.ErrInvalidRollbackTarget
		}

		if !slices.Contains(currentNode.RollbackTargetKeys, targetNode.Key) {
			return shared.ErrInvalidRollbackTarget
		}
	}

	return nil
}

// MergeFormData filters editable form data and merges it into the instance.
func MergeFormData(instance *approval.Instance, formData map[string]any, permissions map[string]approval.Permission) {
	if len(formData) == 0 {
		return
	}

	editableData := FilterEditableFormData(formData, permissions)
	if len(editableData) == 0 {
		return
	}

	if instance.FormData == nil {
		instance.FormData = make(map[string]any, len(editableData))
	}

	maps.Copy(instance.FormData, editableData)
}

// FilterEditableFormData filters form data to only include fields that are editable or required
// based on the node's field permissions configuration.
func FilterEditableFormData(formData map[string]any, permissions map[string]approval.Permission) map[string]any {
	if len(permissions) == 0 {
		return formData
	}

	filtered := make(map[string]any, len(formData))

	for k, v := range formData {
		perm, hasPerm := permissions[k]
		if !hasPerm {
			continue
		}

		if perm == approval.PermissionEditable || perm == approval.PermissionRequired {
			filtered[k] = v
		}
	}

	return filtered
}

// CheckInitiationPermission checks if the applicant is allowed to initiate the flow.
func (s *ValidationService) CheckInitiationPermission(ctx context.Context, db orm.DB, flowID, applicantID string, applicantDeptID *string) (bool, error) {
	var initiators []approval.FlowInitiator

	if err := db.NewSelect().
		Model(&initiators).
		Select("kind", "ids").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", flowID)
		}).
		Scan(ctx); err != nil {
		return false, fmt.Errorf("query flow initiators: %w", err)
	}

	if len(initiators) == 0 {
		return false, nil
	}

	for _, initiator := range initiators {
		switch initiator.Kind {
		case approval.InitiatorUser:
			if slices.Contains(initiator.IDs, applicantID) {
				return true, nil
			}

		case approval.InitiatorDept:
			if applicantDeptID == nil {
				continue
			}

			if slices.Contains(initiator.IDs, *applicantDeptID) {
				return true, nil
			}

		case approval.InitiatorRole:
			if s.assigneeService == nil {
				continue
			}

			for _, roleID := range initiator.IDs {
				users, err := s.assigneeService.GetRoleUsers(ctx, roleID)
				if err != nil {
					return false, fmt.Errorf("get users by role %s: %w", roleID, err)
				}

				if slices.Contains(users, applicantID) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
