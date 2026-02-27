package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
)

// CreateFlowCmd creates a new flow with its initiator configurations.
type CreateFlowCmd struct {
	cqrs.CommandBase
	TenantID              string
	Code                  string
	Name                  string
	CategoryID            string
	Icon                  *string
	Description           *string
	BindingMode           approval.BindingMode
	BusinessTable         *string
	BusinessPkField       *string
	BusinessTitleField    *string
	BusinessStatusField   *string
	AdminUserIDs          []string
	IsAllInitiateAllowed  bool
	InstanceTitleTemplate string
	Initiators            []service.CreateFlowInitiatorCmd
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
	db := dbFromCtx(ctx, h.db)

	tenantID := cmd.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	// Check code uniqueness within tenant
	var existing approval.Flow
	err := db.NewSelect().Model(&existing).Where(func(c orm.ConditionBuilder) {
		c.Equals("tenant_id", tenantID)
		c.Equals("code", cmd.Code)
	}).Scan(ctx)
	if err == nil {
		return nil, service.ErrFlowCodeExists
	}
	if !result.IsRecordNotFound(err) {
		return nil, fmt.Errorf("query flow by code: %w", err)
	}

	flow := approval.Flow{
		TenantID:              tenantID,
		CategoryID:            cmd.CategoryID,
		Code:                  cmd.Code,
		Name:                  cmd.Name,
		Icon:                  cmd.Icon,
		Description:           cmd.Description,
		BindingMode:           cmd.BindingMode,
		BusinessTable:         cmd.BusinessTable,
		BusinessPkField:       cmd.BusinessPkField,
		BusinessTitleField:    cmd.BusinessTitleField,
		BusinessStatusField:   cmd.BusinessStatusField,
		AdminUserIDs:          cmd.AdminUserIDs,
		IsAllInitiateAllowed:  cmd.IsAllInitiateAllowed,
		InstanceTitleTemplate: cmd.InstanceTitleTemplate,
		IsActive:              true,
		CurrentVersion:        0,
	}
	if _, err := db.NewInsert().Model(&flow).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert flow: %w", err)
	}

	for _, init := range cmd.Initiators {
		initiator := approval.FlowInitiator{
			FlowID: flow.ID,
			Kind:   init.Kind,
			IDs:    init.IDs,
		}
		if _, err := db.NewInsert().Model(&initiator).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert flow initiator: %w", err)
		}
	}

	return &flow, nil
}
