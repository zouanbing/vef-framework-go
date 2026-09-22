package strategy

import (
	"context"
	"fmt"
	"strings"

	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/result"
)

// NewUserAssigneeResolver creates a new UserAssigneeResolver.
func NewUserAssigneeResolver() approval.AssigneeResolver {
	return new(UserAssigneeResolver)
}

// UserAssigneeResolver resolves assignees from fixed user IDs.
type UserAssigneeResolver struct{}

func (*UserAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeUser,
		Label:     i18n.T("approval_assignee_kind_user"),
		Selection: approval.SelectionUser,
	}
}

func (*UserAssigneeResolver) Resolve(ctx context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	ids := normalizeIDs(rc.IDs)
	if len(ids) == 0 {
		return nil, nil
	}

	return resolveAssigneesByIDs(ctx, rc, ids, "user assignee resolver")
}

// NewRoleAssigneeResolver creates a new RoleAssigneeResolver.
func NewRoleAssigneeResolver(svc approval.AssigneeService) approval.AssigneeResolver {
	return &RoleAssigneeResolver{svc: svc}
}

// RoleAssigneeResolver resolves assignees from role IDs via AssigneeService.
type RoleAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*RoleAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeRole,
		Label:     i18n.T("approval_assignee_kind_role"),
		Selection: approval.SelectionRole,
	}
}

func (r *RoleAssigneeResolver) Resolve(ctx context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(roleID string) (streams.Stream[approval.ResolvedAssignee], error) {
		users, err := r.svc.GetRoleUsers(ctx, roleID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("role assignee resolver: %w", err)
		}

		return streams.MapTo(streams.FromSlice(users), userInfoToResolvedAssignee), nil
	}))
}

// NewDepartmentAssigneeResolver creates a new DepartmentAssigneeResolver.
func NewDepartmentAssigneeResolver(svc approval.AssigneeService) approval.AssigneeResolver {
	return &DepartmentAssigneeResolver{svc: svc}
}

// DepartmentAssigneeResolver resolves the leaders of the configured
// department IDs (rc.IDs) as assignees.
type DepartmentAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*DepartmentAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeDepartment,
		Label:     i18n.T("approval_assignee_kind_department"),
		Selection: approval.SelectionDepartment,
	}
}

func (r *DepartmentAssigneeResolver) Resolve(ctx context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(departmentID string) (streams.Stream[approval.ResolvedAssignee], error) {
		leaders, err := r.svc.GetDepartmentLeaders(ctx, departmentID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("department assignee resolver: %w", err)
		}

		return streams.MapTo(streams.FromSlice(leaders), userInfoToResolvedAssignee), nil
	}))
}

// NewSelfAssigneeResolver creates a new SelfAssigneeResolver.
func NewSelfAssigneeResolver() approval.AssigneeResolver {
	return new(SelfAssigneeResolver)
}

// SelfAssigneeResolver resolves the applicant as assignee.
type SelfAssigneeResolver struct{}

func (*SelfAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeSelf,
		Label:     i18n.T("approval_assignee_kind_self"),
		Selection: approval.SelectionNone,
	}
}

func (*SelfAssigneeResolver) Resolve(_ context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	applicant := rc.Applicant()
	if applicant.ID == "" {
		return nil, ErrApplicantIDEmpty
	}

	return []approval.ResolvedAssignee{{User: applicant}}, nil
}

// NewSuperiorAssigneeResolver creates a new SuperiorAssigneeResolver.
func NewSuperiorAssigneeResolver(svc approval.AssigneeService) approval.AssigneeResolver {
	return &SuperiorAssigneeResolver{svc: svc}
}

// SuperiorAssigneeResolver resolves the direct superior as assignee.
type SuperiorAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*SuperiorAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeSuperior,
		Label:     i18n.T("approval_assignee_kind_superior"),
		Selection: approval.SelectionNone,
	}
}

func (r *SuperiorAssigneeResolver) Resolve(ctx context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	info, err := r.svc.GetSuperior(ctx, rc.Applicant().ID)
	if err != nil {
		return nil, fmt.Errorf("superior assignee resolver: %w", err)
	}

	if info == nil || info.ID == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	return []approval.ResolvedAssignee{{User: *info}}, nil
}

// NewDepartmentLeaderAssigneeResolver creates a new DepartmentLeaderAssigneeResolver.
func NewDepartmentLeaderAssigneeResolver(svc approval.AssigneeService) approval.AssigneeResolver {
	return &DepartmentLeaderAssigneeResolver{svc: svc}
}

// DepartmentLeaderAssigneeResolver resolves the leaders of the applicant's
// own department as assignees. This is a single-level lookup
// (GetDepartmentLeaders); it does not walk a multi-level supervisor chain.
type DepartmentLeaderAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*DepartmentLeaderAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeDepartmentLeader,
		Label:     i18n.T("approval_assignee_kind_department_leader"),
		Selection: approval.SelectionNone,
	}
}

func (r *DepartmentLeaderAssigneeResolver) Resolve(ctx context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	departmentID := rc.Applicant().DepartmentID
	if departmentID == nil || *departmentID == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	leaders, err := r.svc.GetDepartmentLeaders(ctx, *departmentID)
	if err != nil {
		return nil, fmt.Errorf("department leader assignee resolver: %w", err)
	}

	return streams.MapTo(streams.FromSlice(leaders), userInfoToResolvedAssignee).Collect(), nil
}

// NewFormFieldAssigneeResolver creates a new FormFieldAssigneeResolver.
func NewFormFieldAssigneeResolver() approval.AssigneeResolver {
	return new(FormFieldAssigneeResolver)
}

// FormFieldAssigneeResolver resolves assignees from a form field value.
type FormFieldAssigneeResolver struct{}

func (*FormFieldAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      approval.AssigneeFormField,
		Label:     i18n.T("approval_assignee_kind_form_field"),
		Selection: approval.SelectionFormField,
	}
}

// Resolve reads the assignee IDs from the rule's form field. Unlike the IDs a
// designer picks, these are the applicant's input, so each must name a user
// the host's UserInfoResolver knows: an ID it returns no name for — per the
// resolver contract, one it could not find — fails the action with
// ErrCodeAssigneeResolveFailed instead of becoming a task nobody can see.
func (*FormFieldAssigneeResolver) Resolve(ctx context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	ids, err := approval.FormFieldIDs(rc.FormData, rc.FormField)
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []approval.ResolvedAssignee{}, nil
	}

	assignees, err := resolveAssigneesByIDs(ctx, rc, ids, "form field assignee resolver")
	if err != nil {
		return nil, err
	}

	var unresolved []string

	for _, assignee := range assignees {
		if strings.TrimSpace(assignee.User.Name) == "" {
			unresolved = append(unresolved, assignee.User.ID)
		}
	}

	if len(unresolved) > 0 {
		return nil, result.Err(
			i18n.T(shared.ErrMessageFormFieldAssigneeUnresolved, map[string]any{
				"field": strings.TrimSpace(*rc.FormField),
				"ids":   strings.Join(unresolved, ", "),
			}),
			result.WithCode(approval.ErrCodeAssigneeResolveFailed),
		)
	}

	return assignees, nil
}

// normalizeIDs trims each entry and drops the blanks, preserving order.
func normalizeIDs(raw []string) []string {
	ids := make([]string, 0, len(raw))
	for _, value := range raw {
		if id := strings.TrimSpace(value); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// resolveAssigneesByIDs resolves user info for explicit IDs and converts them
// to assignees, preserving input order. IDs the resolver cannot find still
// yield an assignee (empty name) so a stale reference remains visible instead
// of silently vanishing.
func resolveAssigneesByIDs(ctx context.Context, rc *approval.AssigneeResolveContext, ids []string, label string) ([]approval.ResolvedAssignee, error) {
	infos, err := shared.ResolveUserInfoMap(ctx, rc.UserResolver, ids)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	result := make([]approval.ResolvedAssignee, 0, len(ids))

	for _, userID := range ids {
		info := infos[userID]
		info.ID = userID
		result = append(result, approval.ResolvedAssignee{User: info})
	}

	return result, nil
}

// userInfoToResolvedAssignee converts a UserInfo to a ResolvedAssignee.
func userInfoToResolvedAssignee(info approval.UserInfo) approval.ResolvedAssignee {
	return approval.ResolvedAssignee{User: info}
}

// CompositeAssigneeResolver dispatches each assignee rule to the resolver
// registered for its kind. It owns both the designer-facing option order and
// the runtime index, so the kinds a flow may be saved with and the kinds the
// engine can execute are one set by construction.
type CompositeAssigneeResolver struct {
	ordered   []approval.AssigneeResolver
	resolvers map[approval.AssigneeKind]approval.AssigneeResolver
}

// NewCompositeAssigneeResolver merges host-registered resolvers onto the
// built-ins: a host resolver whose kind matches a built-in replaces it in
// place, any other joins the end of the list.
func NewCompositeAssigneeResolver(builtins, hosts []approval.AssigneeResolver) (*CompositeAssigneeResolver, error) {
	ordered, index, err := overlayKinds(builtins, hosts)
	if err != nil {
		return nil, err
	}

	return &CompositeAssigneeResolver{ordered: ordered, resolvers: index}, nil
}

// Descriptors returns the registered kinds in designer order.
func (c *CompositeAssigneeResolver) Descriptors() []approval.KindDescriptor[approval.AssigneeKind] {
	return describeAll(c.ordered)
}

// Describe returns the descriptor registered for a kind.
func (c *CompositeAssigneeResolver) Describe(kind approval.AssigneeKind) (approval.KindDescriptor[approval.AssigneeKind], bool) {
	resolver, ok := c.resolvers[kind]
	if !ok {
		return approval.KindDescriptor[approval.AssigneeKind]{}, false
	}

	return resolver.Describe(), true
}

// ResolveAll resolves every configured assignee rule of a node in order.
func (c *CompositeAssigneeResolver) ResolveAll(
	ctx context.Context,
	assignees []approval.FlowNodeAssignee,
	base *approval.NodeResolveContext,
) ([]approval.ResolvedAssignee, error) {
	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(assignees), func(assignee approval.FlowNodeAssignee) (streams.Stream[approval.ResolvedAssignee], error) {
		resolver, ok := c.resolvers[assignee.Kind]
		if !ok {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("%w: %s", ErrAssigneeResolverNotFound, assignee.Kind)
		}

		resolved, err := resolver.Resolve(ctx, &approval.AssigneeResolveContext{
			NodeResolveContext: *base,
			Kind:               assignee.Kind,
			IDs:                assignee.IDs,
			FormField:          assignee.FormField,
		})
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("composite assignee resolver %q: %w", assignee.Kind, err)
		}

		return streams.FromSlice(resolved), nil
	}))
}
