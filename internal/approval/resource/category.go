package resource

import (
	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/crud"
)

// CategoryParams contains the create/update parameters for flow category.
type CategoryParams struct {
	api.P

	ID        string  `json:"id"`
	TenantID  string  `json:"tenantId" validate:"required"`
	Code      string  `json:"code" validate:"required"`
	Name      string  `json:"name" validate:"required"`
	Icon      *string `json:"icon"`
	ParentID  *string `json:"parentId"`
	SortOrder int     `json:"sortOrder"`
	IsActive  bool    `json:"isActive"`
	Remark    *string `json:"remark"`
}

// CategorySearch contains the search parameters for flow category.
type CategorySearch struct {
	crud.Sortable

	Name     string `json:"name" search:"contains"`
	IsActive *bool  `json:"isActive" search:"eq,column=is_active"`
}

// CategoryResource handles flow category CRUD using standard apis generics.
type CategoryResource struct {
	api.Resource

	crud.FindAll[approval.FlowCategory, CategorySearch]
	crud.FindPage[approval.FlowCategory, CategorySearch]
	crud.Create[approval.FlowCategory, CategoryParams]
	crud.Update[approval.FlowCategory, CategoryParams]
	crud.Delete[approval.FlowCategory]
}

// NewCategoryResource creates a new category resource with standard CRUD operations.
func NewCategoryResource() api.Resource {
	return &CategoryResource{
		Resource: api.NewRPCResource("approval/category"),
		FindAll:  crud.NewFindAll[approval.FlowCategory, CategorySearch](),
		FindPage: crud.NewFindPage[approval.FlowCategory, CategorySearch](),
		Create:   crud.NewCreate[approval.FlowCategory, CategoryParams](),
		Update:   crud.NewUpdate[approval.FlowCategory, CategoryParams](),
		Delete:   crud.NewDelete[approval.FlowCategory](),
	}
}
