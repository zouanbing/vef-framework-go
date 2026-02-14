package orm_test

import (
	"context"
	"database/sql"

	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/orm"
)

// User represents a user in the system.
type User struct {
	bun.BaseModel `bun:"table:test_user,alias:u"`
	orm.Model

	Name     string         `json:"name"     bun:"name,notnull"`
	Email    string         `json:"email"    bun:"email,notnull,unique"`
	Age      int16          `json:"age"      bun:"age,notnull,default:0"`
	IsActive bool           `json:"isActive" bun:"is_active,notnull"`
	Meta     map[string]any `json:"meta"     bun:"meta"`

	// Relations
	Posts []Post `json:"posts" bun:"rel:has-many,join:id=user_id"`
}

// Post represents a blog post or article.
type Post struct {
	bun.BaseModel `bun:"table:test_post,alias:p"`
	orm.Model

	Title       string  `json:"title"       bun:"title,notnull"`
	Content     string  `json:"content"     bun:"content,notnull"`
	Description *string `json:"description" bun:"description"`
	UserID      string  `json:"userId"      bun:"user_id,notnull"`
	CategoryID  string  `json:"categoryId"  bun:"category_id,notnull"`
	Status      string  `json:"status"      bun:"status,notnull,default:'draft'"`
	ViewCount   int     `json:"viewCount"   bun:"view_count,notnull,default:0"`

	// Relations
	User     *User     `json:"user"     bun:"rel:belongs-to,join:user_id=id"`
	Category *Category `json:"category" bun:"rel:belongs-to,join:category_id=id"`
}

// Tag represents a content tag.
type Tag struct {
	bun.BaseModel `bun:"table:test_tag,alias:t"`
	orm.Model

	Name        string  `json:"name"        bun:"name,notnull,unique"`
	Description *string `json:"description" bun:"description"`
}

// PostTag represents the many-to-many relationship between posts and tags.
type PostTag struct {
	bun.BaseModel `bun:"table:test_post_tag,alias:pt"`
	orm.Model

	PostID string `json:"postId" bun:"post_id,notnull"`
	TagID  string `json:"tagId"  bun:"tag_id,notnull"`

	// Relations
	Post *Post `json:"post" bun:"rel:belongs-to,join:post_id=id"`
	Tag  *Tag  `json:"tag"  bun:"rel:belongs-to,join:tag_id=id"`
}

// Category represents a content category.
type Category struct {
	bun.BaseModel `bun:"table:test_category,alias:c"`
	orm.Model

	Name        string  `json:"name"        bun:"name,notnull,unique"`
	Description *string `json:"description" bun:"description"`
	ParentID    *string `json:"parentId"    bun:"parent_id"`

	// Relations
	Posts    []Post     `json:"posts"    bun:"rel:has-many,join:id=category_id"`
	Parent   *Category  `json:"parent"   bun:"rel:belongs-to,join:parent_id=id"`
	Children []Category `json:"children" bun:"rel:has-many,join:id=parent_id"`
}

// BaseTestSuite contains all the actual test methods and works with orm.DB interface.
// This suite will be run against multiple databases to verify cross-database compatibility.
type BaseTestSuite struct {
	suite.Suite

	ctx   context.Context
	db    orm.DB
	rawDB *sql.DB
	ds    *config.DataSourceConfig
}

// SetupSuite initializes the test suite (called once per database).
func (suite *BaseTestSuite) SetupSuite() {
	models := []any{
		(*User)(nil),
		(*Post)(nil),
		(*Tag)(nil),
		(*PostTag)(nil),
		(*Category)(nil),
	}

	suite.db.RegisterModel(models...)
	suite.Require().NoError(suite.db.ResetModel(suite.ctx, models...), "Failed to reset models")

	fixtures, err := testfixtures.New(
		testfixtures.Database(suite.rawDB),
		testfixtures.Dialect(string(suite.ds.Kind)),
		testfixtures.Directory("fixtures"),
		testfixtures.DangerousSkipTestDatabaseCheck(),
	)
	suite.Require().NoError(err, "Failed to create fixtures loader")
	suite.Require().NoError(fixtures.Load(), "Failed to load fixtures")
}
