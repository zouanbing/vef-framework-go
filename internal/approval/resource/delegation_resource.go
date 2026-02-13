package resource

import (
	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/apis"
	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/null"
)

// DelegationParams contains the create/update parameters for delegation.
type DelegationParams struct {
	api.P

	ID             string      `json:"id"`
	DelegatorID    string      `json:"delegatorId" validate:"required"`
	DelegateeID    string      `json:"delegateeId" validate:"required"`
	FlowCategoryID null.String `json:"flowCategoryId"`
	FlowID         null.String `json:"flowId"`
	StartTime      null.Time   `json:"startTime"`
	EndTime        null.Time   `json:"endTime"`
	IsActive       bool        `json:"isActive"`
	Reason         null.String `json:"reason"`
}

// DelegationSearch contains the search parameters for delegation.
type DelegationSearch struct {
	apis.Sortable

	DelegatorID string `json:"delegatorId" search:"eq,column=delegator_id"`
	DelegateeID string `json:"delegateeId" search:"eq,column=delegatee_id"`
	IsActive    *bool  `json:"isActive" search:"eq,column=is_active"`
}

// DelegationResource handles delegation CRUD using standard apis generics.
type DelegationResource struct {
	api.Resource

	apis.FindAll[approval.Delegation, DelegationSearch]
	apis.FindPage[approval.Delegation, DelegationSearch]
	apis.Create[approval.Delegation, DelegationParams]
	apis.Update[approval.Delegation, DelegationParams]
	apis.Delete[approval.Delegation]
}

// NewDelegationResource creates a new delegation resource with standard CRUD operations.
func NewDelegationResource() *DelegationResource {
	return &DelegationResource{
		Resource: api.NewRPCResource("approval/delegation"),
		FindAll:  apis.NewFindAll[approval.Delegation, DelegationSearch](),
		FindPage: apis.NewFindPage[approval.Delegation, DelegationSearch](),
		Create:   apis.NewCreate[approval.Delegation, DelegationParams](),
		Update:   apis.NewUpdate[approval.Delegation, DelegationParams](),
		Delete:   apis.NewDelete[approval.Delegation](),
	}
}
