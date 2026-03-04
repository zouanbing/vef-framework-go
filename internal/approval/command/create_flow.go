package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// CreateFlowCmd creates a new flow with its initiator configurations.
type CreateFlowCmd struct {
	cqrs.BaseCommand

	TenantID               string
	Code                   string
	Name                   string
	CategoryID             string
	Icon                   *string
	Description            *string
	BindingMode            approval.BindingMode
	BusinessTable          *string
	BusinessPkField        *string
	BusinessTitleField     *string
	BusinessStatusField    *string
	AdminUserIDs           []string
	IsAllInitiationAllowed bool
	InstanceTitleTemplate  string
	Initiators             []shared.CreateFlowInitiatorCmd
}

// CreateFlowHandler handles the CreateFlowCmd command.
type CreateFlowHandler struct {
	db orm.DB
}

// NewCreateFlowHandler creates a new CreateFlowHandler.
func NewCreateFlowHandler(db orm.DB) *CreateFlowHandler {
	return &CreateFlowHandler{db: db}
}

func (h *CreateFlowHandler) Handle(ctx context.Context, cmd CreateFlowCmd) (*approval.Flow, error) {
	db := contextx.DB(ctx, h.db)

	tenantID := cmd.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	// Check code uniqueness within tenant
	var existing approval.Flow
	exists, err := db.NewSelect().
		Model(&existing).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("tenant_id", tenantID).
				Equals("code", cmd.Code)
		}).
		Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("query flow by code: %w", err)
	}
	if exists {
		return nil, shared.ErrFlowCodeExists
	}

	flow := approval.Flow{
		TenantID:               tenantID,
		CategoryID:             cmd.CategoryID,
		Code:                   cmd.Code,
		Name:                   cmd.Name,
		Icon:                   cmd.Icon,
		Description:            cmd.Description,
		BindingMode:            cmd.BindingMode,
		BusinessTable:          cmd.BusinessTable,
		BusinessPkField:        cmd.BusinessPkField,
		BusinessTitleField:     cmd.BusinessTitleField,
		BusinessStatusField:    cmd.BusinessStatusField,
		AdminUserIDs:           cmd.AdminUserIDs,
		IsAllInitiationAllowed: cmd.IsAllInitiationAllowed,
		InstanceTitleTemplate:  cmd.InstanceTitleTemplate,
		IsActive:               true,
		CurrentVersion:         0,
	}
	if _, err := db.NewInsert().
		Model(&flow).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert flow: %w", err)
	}

	for _, init := range cmd.Initiators {
		initiator := approval.FlowInitiator{
			FlowID: flow.ID,
			Kind:   init.Kind,
			IDs:    init.IDs,
		}
		if _, err := db.NewInsert().
			Model(&initiator).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert flow initiator: %w", err)
		}
	}

	return &flow, nil
}
