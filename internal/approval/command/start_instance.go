package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// StartInstanceCmd starts a new approval flow instance.
type StartInstanceCmd struct {
	cqrs.BaseCommand
	TenantID         string
	FlowCode         string
	ApplicantID      string
	ApplicantDeptID  *string
	BusinessRecordID *string
	FormData         map[string]any
}

// StartInstanceHandler handles the StartInstanceCmd command.
type StartInstanceHandler struct {
	db            orm.DB
	engine        *engine.FlowEngine
	instanceNoGen service.InstanceNoGenerator
	publisher     *dispatcher.EventPublisher
	validSvc      *service.ValidationService
	flowSvc       *service.FlowService
}

// NewStartInstanceHandler creates a new StartInstanceHandler.
func NewStartInstanceHandler(
	db orm.DB,
	eng *engine.FlowEngine,
	instanceNoGen service.InstanceNoGenerator,
	pub *dispatcher.EventPublisher,
	validSvc *service.ValidationService,
	flowSvc *service.FlowService,
) *StartInstanceHandler {
	return &StartInstanceHandler{
		db:            db,
		engine:        eng,
		instanceNoGen: instanceNoGen,
		publisher:     pub,
		validSvc:      validSvc,
		flowSvc:       flowSvc,
	}
}

func (h *StartInstanceHandler) Handle(ctx context.Context, cmd StartInstanceCmd) (*approval.Instance, error) {
	db := contextx.DB(ctx, h.db)

	tenantID := cmd.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	var flow approval.Flow
	if err := db.NewSelect().Model(&flow).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("tenant_id", tenantID)
		cb.Equals("code", cmd.FlowCode)
	}).Scan(ctx); err != nil {
		return nil, service.ErrFlowNotFound
	}

	if !flow.IsActive {
		return nil, service.ErrFlowNotActive
	}

	if !flow.IsAllInitiateAllowed {
		allowed, err := h.validSvc.CheckInitiationPermission(ctx, db, flow.ID, cmd.ApplicantID, cmd.ApplicantDeptID)
		if err != nil {
			return nil, fmt.Errorf("check initiation permission: %w", err)
		}
		if !allowed {
			return nil, service.ErrNotAllowedInitiate
		}
	}

	var version approval.FlowVersion
	if err := db.NewSelect().Model(&version).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("flow_id", flow.ID)
		cb.Equals("status", string(approval.VersionPublished))
	}).Scan(ctx); err != nil {
		return nil, service.ErrNoPublishedVersion
	}

	instanceNo, err := h.instanceNoGen.Generate(ctx, cmd.FlowCode)
	if err != nil {
		return nil, fmt.Errorf("generate instance number: %w", err)
	}

	title, err := h.flowSvc.RenderTitle(
		flow.InstanceTitleTemplate,
		h.flowSvc.BuildTitleTemplateData(flow.Name, flow.Code, instanceNo, cmd.FormData),
	)
	if err != nil {
		return nil, fmt.Errorf("render instance title: %w", err)
	}

	instance := &approval.Instance{
		TenantID:         flow.TenantID,
		FlowID:           flow.ID,
		FlowVersionID:    version.ID,
		Title:            title,
		InstanceNo:       instanceNo,
		ApplicantID:      cmd.ApplicantID,
		ApplicantDeptID:  cmd.ApplicantDeptID,
		Status:           approval.InstanceRunning,
		BusinessRecordID: cmd.BusinessRecordID,
		FormData:         cmd.FormData,
	}

	if _, err := db.NewInsert().Model(instance).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert instance: %w", err)
	}

	submitLog := &approval.ActionLog{
		InstanceID: instance.ID,
		Action:     approval.ActionSubmit,
		OperatorID: cmd.ApplicantID,
	}
	if _, err := db.NewInsert().Model(submitLog).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert submit log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewInstanceCreatedEvent(instance.ID, flow.ID, title, cmd.ApplicantID),
	}); err != nil {
		return nil, fmt.Errorf("publish instance created event: %w", err)
	}

	if err := h.engine.StartProcess(ctx, db, instance); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	if _, err := db.NewUpdate().Model(instance).WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("update instance after start: %w", err)
	}

	return instance, nil
}
