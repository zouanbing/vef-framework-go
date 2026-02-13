package resource

import (
	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/apis"
	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/null"
)

// CategoryParams contains the create/update parameters for flow category.
type CategoryParams struct {
	api.P

	ID        string      `json:"id"`
	Code      string      `json:"code" validate:"required"`
	Name      string      `json:"name" validate:"required"`
	Icon      null.String `json:"icon"`
	ParentID  null.String `json:"parentId"`
	SortOrder int         `json:"sortOrder"`
	IsActive  bool        `json:"isActive"`
	Remark    null.String `json:"remark"`
}

// CategorySearch contains the search parameters for flow category.
type CategorySearch struct {
	apis.Sortable

	Name     string `json:"name" search:"contains"`
	IsActive *bool  `json:"isActive" search:"eq,column=is_active"`
}

// CategoryResource handles flow category CRUD using standard apis generics.
type CategoryResource struct {
	api.Resource

	apis.FindAll[approval.FlowCategory, CategorySearch]
	apis.FindPage[approval.FlowCategory, CategorySearch]
	apis.Create[approval.FlowCategory, CategoryParams]
	apis.Update[approval.FlowCategory, CategoryParams]
	apis.Delete[approval.FlowCategory]
}

// NewCategoryResource creates a new category resource with standard CRUD operations.
func NewCategoryResource() *CategoryResource {
	return &CategoryResource{
		Resource: api.NewRPCResource("approval/category"),
		FindAll:  apis.NewFindAll[approval.FlowCategory, CategorySearch](),
		FindPage: apis.NewFindPage[approval.FlowCategory, CategorySearch](),
		Create:   apis.NewCreate[approval.FlowCategory, CategoryParams](),
		Update:   apis.NewUpdate[approval.FlowCategory, CategoryParams](),
		Delete:   apis.NewDelete[approval.FlowCategory](),
	}
}
