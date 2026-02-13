package strategy

import (
	"context"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/orm"
)

// ResolveContext provides context for assignee resolution.
type ResolveContext struct {
	DB          orm.DB
	ApplicantID string
	DeptID      string
	FormData    approval.FormData
	Config      *approval.FlowNodeAssignee
	OrgService  approval.OrganizationService
	UserService approval.UserService
}

// AssigneeResolver resolves assignees for a specific kind.
type AssigneeResolver interface {
	Kind() approval.AssigneeKind
	Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error)
}

// NewUserResolver creates a new UserResolver.
func NewUserResolver() AssigneeResolver {
	return new(UserResolver)
}

// UserResolver resolves assignees from fixed user IDs.
type UserResolver struct{}

func (r *UserResolver) Kind() approval.AssigneeKind { return approval.AssigneeUser }

func (r *UserResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	result := make([]approval.ResolvedAssignee, 0, len(rc.Config.AssigneeIDs))
	for _, uid := range rc.Config.AssigneeIDs {
		result = append(result, approval.ResolvedAssignee{UserID: uid})
	}

	return result, nil
}

// NewRoleResolver creates a new RoleResolver.
func NewRoleResolver() AssigneeResolver {
	return new(RoleResolver)
}

// RoleResolver resolves assignees from role IDs via UserService.
type RoleResolver struct{}

func (r *RoleResolver) Kind() approval.AssigneeKind { return approval.AssigneeRole }

func (r *RoleResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.UserService == nil {
		return nil, nil
	}

	var result []approval.ResolvedAssignee
	for _, roleID := range rc.Config.AssigneeIDs {
		users, err := rc.UserService.GetUsersByRole(ctx, roleID)
		if err != nil {
			return nil, err
		}

		for _, uid := range users {
			result = append(result, approval.ResolvedAssignee{UserID: uid})
		}
	}

	return result, nil
}

// NewDeptResolver creates a new DeptResolver.
func NewDeptResolver() AssigneeResolver {
	return new(DeptResolver)
}

// DeptResolver resolves department leaders as assignees.
type DeptResolver struct{}

func (r *DeptResolver) Kind() approval.AssigneeKind { return approval.AssigneeDept }

func (r *DeptResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.OrgService == nil {
		return nil, nil
	}

	var result []approval.ResolvedAssignee
	for _, deptID := range rc.Config.AssigneeIDs {
		leaders, err := rc.OrgService.GetDeptLeaders(ctx, deptID)
		if err != nil {
			return nil, err
		}

		for _, uid := range leaders {
			result = append(result, approval.ResolvedAssignee{UserID: uid})
		}
	}

	return result, nil
}

// NewSelfResolver creates a new SelfResolver.
func NewSelfResolver() AssigneeResolver {
	return new(SelfResolver)
}

// SelfResolver resolves the applicant as assignee.
type SelfResolver struct{}

func (r *SelfResolver) Kind() approval.AssigneeKind { return approval.AssigneeSelf }

func (r *SelfResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.ApplicantID == "" {
		return nil, nil
	}

	return []approval.ResolvedAssignee{{UserID: rc.ApplicantID}}, nil
}

// NewSuperiorResolver creates a new SuperiorResolver.
func NewSuperiorResolver() AssigneeResolver {
	return new(SuperiorResolver)
}

// SuperiorResolver resolves the direct superior as assignee.
type SuperiorResolver struct{}

func (r *SuperiorResolver) Kind() approval.AssigneeKind { return approval.AssigneeSuperior }

func (r *SuperiorResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.OrgService == nil {
		return nil, nil
	}

	uid, _, err := rc.OrgService.GetSuperior(ctx, rc.ApplicantID)
	if err != nil {
		return nil, err
	}

	if uid == "" {
		return nil, nil
	}

	return []approval.ResolvedAssignee{{UserID: uid}}, nil
}

// NewDeptLeaderResolver creates a new DeptLeaderResolver.
func NewDeptLeaderResolver() AssigneeResolver {
	return new(DeptLeaderResolver)
}

// DeptLeaderResolver resolves department leaders as assignees.
type DeptLeaderResolver struct{}

func (r *DeptLeaderResolver) Kind() approval.AssigneeKind { return approval.AssigneeDeptLeader }

func (r *DeptLeaderResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.OrgService == nil {
		return nil, nil
	}

	leaders, err := rc.OrgService.GetDeptLeaders(ctx, rc.DeptID)
	if err != nil {
		return nil, err
	}

	result := make([]approval.ResolvedAssignee, 0, len(leaders))
	for _, uid := range leaders {
		result = append(result, approval.ResolvedAssignee{UserID: uid})
	}

	return result, nil
}

// NewFormFieldResolver creates a new FormFieldResolver.
func NewFormFieldResolver() AssigneeResolver {
	return new(FormFieldResolver)
}

// FormFieldResolver resolves assignees from a form field value.
type FormFieldResolver struct{}

func (r *FormFieldResolver) Kind() approval.AssigneeKind { return approval.AssigneeFormField }

func (r *FormFieldResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	fieldName := rc.Config.FormField.String
	if fieldName == "" {
		return nil, nil
	}

	value := rc.FormData.Get(fieldName)

	switch v := value.(type) {
	case string:
		if v == "" {
			return nil, nil
		}

		return []approval.ResolvedAssignee{{UserID: v}}, nil
	case []string:
		result := make([]approval.ResolvedAssignee, 0, len(v))
		for _, uid := range v {
			result = append(result, approval.ResolvedAssignee{UserID: uid})
		}

		return result, nil
	case []any:
		result := make([]approval.ResolvedAssignee, 0, len(v))
		for _, item := range v {
			if uid, ok := item.(string); ok && uid != "" {
				result = append(result, approval.ResolvedAssignee{UserID: uid})
			}
		}

		return result, nil
	default:
		return nil, nil
	}
}

// CompositeResolver chains multiple resolvers and resolves assignees based on config kind.
type CompositeResolver struct {
	resolvers map[approval.AssigneeKind]AssigneeResolver
}

// NewCompositeResolver creates a composite resolver from individual resolvers.
func NewCompositeResolver(resolvers ...AssigneeResolver) *CompositeResolver {
	m := make(map[approval.AssigneeKind]AssigneeResolver, len(resolvers))
	for _, r := range resolvers {
		m[r.Kind()] = r
	}

	return &CompositeResolver{resolvers: m}
}

// ResolveAll resolves assignees from multiple configs, deduplicating by user ID.
func (c *CompositeResolver) ResolveAll(ctx context.Context, configs []*approval.FlowNodeAssignee, baseCtx *ResolveContext) ([]approval.ResolvedAssignee, error) {
	var all []approval.ResolvedAssignee

	for _, cfg := range configs {
		resolver, ok := c.resolvers[cfg.AssigneeKind]
		if !ok {
			continue
		}

		rc := *baseCtx
		rc.Config = cfg

		assignees, err := resolver.Resolve(ctx, &rc)
		if err != nil {
			return nil, err
		}

		all = append(all, assignees...)
	}

	return all, nil
}
