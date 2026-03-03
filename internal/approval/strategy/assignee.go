package strategy

import (
	"context"
	"fmt"

	streams "github.com/ilxqx/go-streams"
	"github.com/spf13/cast"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/orm"
)

// ResolveContext provides context for assignee resolution.
type ResolveContext struct {
	DB              orm.DB
	ApplicantID     string
	ApplicantDeptID *string
	FormData        approval.FormData

	IDs       []string
	FormField *string
}

// AssigneeResolver resolves assignees for a specific kind.
type AssigneeResolver interface {
	// Kind returns the assignee kind this resolver handles (user, role, dept_leader, superior, etc.).
	Kind() approval.AssigneeKind
	// Resolve resolves concrete assignees from the assignee configuration in ResolveContext.
	Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error)
}

// NewUserAssigneeResolver creates a new UserAssigneeResolver.
func NewUserAssigneeResolver() AssigneeResolver {
	return new(UserAssigneeResolver)
}

// UserAssigneeResolver resolves assignees from fixed user IDs.
type UserAssigneeResolver struct{}

func (r *UserAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeUser }

func (r *UserAssigneeResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	result := make([]approval.ResolvedAssignee, len(rc.IDs))
	for i, uid := range rc.IDs {
		result[i] = approval.ResolvedAssignee{UserID: uid}
	}

	return result, nil
}

// NewRoleAssigneeResolver creates a new RoleAssigneeResolver.
func NewRoleAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &RoleAssigneeResolver{svc: svc}
}

// RoleAssigneeResolver resolves assignees from role IDs via AssigneeService.
type RoleAssigneeResolver struct {
	svc approval.AssigneeService
}

func (r *RoleAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeRole }

func (r *RoleAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(roleID string) (streams.Stream[approval.ResolvedAssignee], error) {
		users, err := r.svc.GetRoleUsers(ctx, roleID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("role assignee resolver: %w", err)
		}

		return streams.MapTo(streams.FromSlice(users), func(uid string) approval.ResolvedAssignee {
			return approval.ResolvedAssignee{UserID: uid}
		}), nil
	}))
}

// NewDeptAssigneeResolver creates a new DeptAssigneeResolver.
func NewDeptAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &DeptAssigneeResolver{svc: svc}
}

// DeptAssigneeResolver resolves department leaders as assignees.
type DeptAssigneeResolver struct {
	svc approval.AssigneeService
}

func (r *DeptAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeDept }

func (r *DeptAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(deptID string) (streams.Stream[approval.ResolvedAssignee], error) {
		leaders, err := r.svc.GetDeptLeaders(ctx, deptID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("dept assignee resolver: %w", err)
		}

		return streams.MapTo(streams.FromSlice(leaders), func(uid string) approval.ResolvedAssignee {
			return approval.ResolvedAssignee{UserID: uid}
		}), nil
	}))
}

// NewSelfAssigneeResolver creates a new SelfAssigneeResolver.
func NewSelfAssigneeResolver() AssigneeResolver {
	return new(SelfAssigneeResolver)
}

// SelfAssigneeResolver resolves the applicant as assignee.
type SelfAssigneeResolver struct{}

func (r *SelfAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeSelf }

func (r *SelfAssigneeResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.ApplicantID == "" {
		return nil, ErrApplicantIDEmpty
	}

	return []approval.ResolvedAssignee{{UserID: rc.ApplicantID}}, nil
}

// NewSuperiorAssigneeResolver creates a new SuperiorAssigneeResolver.
func NewSuperiorAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &SuperiorAssigneeResolver{svc: svc}
}

// SuperiorAssigneeResolver resolves the direct superior as assignee.
type SuperiorAssigneeResolver struct {
	svc approval.AssigneeService
}

func (r *SuperiorAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeSuperior }

func (r *SuperiorAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	uid, err := r.svc.GetSuperior(ctx, rc.ApplicantID)
	if err != nil {
		return nil, fmt.Errorf("superior assignee resolver: %w", err)
	}

	if uid == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	return []approval.ResolvedAssignee{{UserID: uid}}, nil
}

// NewDeptLeaderAssigneeResolver creates a new DeptLeaderAssigneeResolver.
func NewDeptLeaderAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &DeptLeaderAssigneeResolver{svc: svc}
}

// DeptLeaderAssigneeResolver resolves department leaders as assignees.
type DeptLeaderAssigneeResolver struct {
	svc approval.AssigneeService
}

func (r *DeptLeaderAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeDeptLeader }

func (r *DeptLeaderAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	if rc.ApplicantDeptID == nil || *rc.ApplicantDeptID == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	leaders, err := r.svc.GetDeptLeaders(ctx, *rc.ApplicantDeptID)
	if err != nil {
		return nil, fmt.Errorf("dept leader assignee resolver: %w", err)
	}

	result := make([]approval.ResolvedAssignee, len(leaders))
	for i, uid := range leaders {
		result[i] = approval.ResolvedAssignee{UserID: uid}
	}

	return result, nil
}

// NewFormFieldAssigneeResolver creates a new FormFieldAssigneeResolver.
func NewFormFieldAssigneeResolver() AssigneeResolver {
	return new(FormFieldAssigneeResolver)
}

// FormFieldAssigneeResolver resolves assignees from a form field value.
type FormFieldAssigneeResolver struct{}

func (r *FormFieldAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeFormField }

func (r *FormFieldAssigneeResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.FormField == nil || *rc.FormField == "" {
		return nil, ErrFormFieldNameEmpty
	}

	value := rc.FormData.Get(*rc.FormField)

	switch v := value.(type) {
	case nil:
		return []approval.ResolvedAssignee{}, nil
	case string:
		if v == "" {
			return nil, ErrFormFieldValueEmpty
		}

		return []approval.ResolvedAssignee{{UserID: v}}, nil
	case []string:
		result := make([]approval.ResolvedAssignee, len(v))
		for i, uid := range v {
			result[i] = approval.ResolvedAssignee{UserID: uid}
		}

		return result, nil
	case []any:
		result := make([]approval.ResolvedAssignee, len(v))
		for i, item := range v {
			if uid := cast.ToString(item); uid != "" {
				result[i] = approval.ResolvedAssignee{UserID: uid}
			} else {
				return nil, ErrFormFieldValueEmpty
			}
		}

		return result, nil
	default:
		return nil, fmt.Errorf("%w: %T", ErrUnsupportedFieldValueType, value)
	}
}

// CompositeAssigneeResolver chains multiple resolvers and resolves assignees based on config kind.
type CompositeAssigneeResolver struct {
	resolvers map[approval.AssigneeKind]AssigneeResolver
}

// NewCompositeAssigneeResolver creates a composite resolver from individual resolvers.
func NewCompositeAssigneeResolver(resolvers ...AssigneeResolver) *CompositeAssigneeResolver {
	return &CompositeAssigneeResolver{
		resolvers: streams.AssociateBy(streams.FromSlice(resolvers), func(r AssigneeResolver) approval.AssigneeKind {
			return r.Kind()
		}),
	}
}

// ResolveAll resolves assignees from multiple configs.
func (c *CompositeAssigneeResolver) ResolveAll(ctx context.Context, assignees []approval.FlowNodeAssignee, baseRC *ResolveContext) ([]approval.ResolvedAssignee, error) {
	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(assignees), func(assignee approval.FlowNodeAssignee) (streams.Stream[approval.ResolvedAssignee], error) {
		resolver, ok := c.resolvers[assignee.Kind]
		if !ok {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("%w: %s", ErrAssigneeResolverNotFound, assignee.Kind)
		}

		rc := *baseRC
		rc.IDs = assignee.IDs
		rc.FormField = assignee.FormField

		resolved, err := resolver.Resolve(ctx, &rc)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("composite assignee resolver %q: %w", assignee.Kind, err)
		}

		return streams.FromSlice(resolved), nil
	}))
}
