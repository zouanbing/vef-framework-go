package orm_test

import (
	"errors"
	"fmt"

	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun/schema"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBBasicExpressionsTestSuite{BaseTestSuite: base}
	})
}

// EBBasicExpressionsTestSuite tests basic expression methods of orm.ExprBuilder
// including Column, TableColumns, AllColumns, Null, IsNull, IsNotNull, Literal, Order, Case, Expr, Exprs, ExprsWithSep.
type EBBasicExpressionsTestSuite struct {
	*BaseTestSuite
}

func (suite *EBBasicExpressionsTestSuite) TestColumn() {
	suite.Run("SimpleColumnReference", func() {
		type Result struct {
			ID    string `bun:"id"`
			Title string `bun:"title"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("id")
			}, "id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("title")
			}, "title").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
		}
	})

	suite.Run("TableQualifiedColumnReference", func() {
		type Result struct {
			PostID    string `bun:"post_id"`
			PostTitle string `bun:"post_title"`
			UserName  string `bun:"user_name"`
		}

		var results []Result

		err := suite.selectPosts().
			Join((*User)(nil), func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "p.user_id")
			}).
			SelectAs("p.id", "post_id").
			SelectAs("p.title", "post_title").
			SelectAs("u.name", "user_name").
			OrderBy("p.id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.PostID)
			suite.NotEmpty(result.PostTitle)
			suite.NotEmpty(result.UserName)
		}
	})

	suite.Run("ColumnWithTableAliasTrue", func() {
		type Result struct {
			ID    string `bun:"id"`
			Title string `bun:"title"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("id", true)
			}, "id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("title", true)
			}, "title").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
		}
	})

	suite.Run("ColumnWithTableAliasFalse", func() {
		type Result struct {
			ID    string `bun:"id"`
			Title string `bun:"title"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("id", false)
			}, "id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("title", false)
			}, "title").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestTableColumns() {
	suite.Run("TableColumnsWithDefaultAlias", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
			UserID    string `bun:"user_id"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.TableColumns()
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
			suite.NotEmpty(result.UserID)
		}
	})

	suite.Run("TableColumnsWithAliasTrue", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.TableColumns(true)
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
		}
	})

	suite.Run("TableColumnsWithoutAlias", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.TableColumns(false)
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestAllColumns() {
	suite.Run("AllColumnsWithDefaultBehavior", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
			UserID    string `bun:"user_id"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AllColumns()
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
			suite.NotEmpty(result.UserID)
		}
	})

	suite.Run("AllColumnsWithExplicitAlias", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AllColumns("p")
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
		}
	})

	suite.Run("AllColumnsWithEmptyAlias", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AllColumns("")
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
		}
	})

	suite.Run("AllColumnsCombinedWithExpressions", func() {
		type Result struct {
			ID          string `bun:"id"`
			Title       string `bun:"title"`
			Status      string `bun:"status"`
			ViewCount   int64  `bun:"view_count"`
			UserID      string `bun:"user_id"`
			DoubleViews int64  `bun:"double_views"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AllColumns()
			}).
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Multiply(eb.Column("view_count"), 2)
			}, "double_views").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
			suite.NotEmpty(result.UserID)
			suite.Equal(result.ViewCount*2, result.DoubleViews)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestNull() {
	suite.Run("NullValueInSelect", func() {
		type Result struct {
			ID        string  `bun:"id"`
			Title     string  `bun:"title"`
			NullValue *string `bun:"null_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Null()
			}, "null_value").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.Nil(result.NullValue)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestIsNull() {
	suite.Run("CheckNullValues", func() {
		type Result struct {
			ID     string `bun:"id"`
			Status string `bun:"status"`
			IsNull bool   `bun:"is_null"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.IsNull(eb.Expr("CASE WHEN ? = 'active' THEN NULL ELSE ? END",
					eb.Column("status"), eb.Column("status")))
			}, "is_null").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)
	})

	suite.Run("IsNullWithCoalesce", func() {
		type Result struct {
			ID        string `bun:"id"`
			Status    string `bun:"status"`
			SafeValue string `bun:"safe_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Coalesce(
					eb.Expr("CASE WHEN ? = 'active' THEN NULL ELSE ? END",
						eb.Column("status"), eb.Column("status")),
					"default",
				)
			}, "safe_value").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.SafeValue, "Safe value should never be empty due to Coalesce")
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestIsNotNull() {
	suite.Run("CheckNotNullValues", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			IsNotNull bool   `bun:"is_not_null"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.IsNotNull(eb.Column("title"))
			}, "is_not_null").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.True(result.IsNotNull, "Title should not be NULL for existing posts")
		}
	})

	suite.Run("CombinedNullChecks", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			HasTitle  bool   `bun:"has_title"`
			HasStatus bool   `bun:"has_status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.IsNotNull(eb.Column("title"))
			}, "has_title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.IsNotNull(eb.Column("status"))
			}, "has_status").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.Title)
			suite.NotEmpty(result.Status)
			suite.True(result.HasTitle)
			suite.True(result.HasStatus)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestLiteral() {
	suite.Run("LiteralValuesInExpressions", func() {
		type Result struct {
			ID         string `bun:"id"`
			Title      string `bun:"title"`
			LiteralVal string `bun:"literal_val"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Literal("test_literal")
			}, "literal_val").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.Equal("test_literal", result.LiteralVal)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestOrder() {
	suite.Run("SimpleOrderBy", func() {
		type Result struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
			Age  int16  `bun:"age"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "age").
			OrderByExpr(func(eb orm.ExprBuilder) any {
				return eb.Order(func(ob orm.OrderBuilder) {
					ob.Column("age").Desc()
				})
			}).
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for i := 1; i < len(results); i++ {
			suite.True(results[i].Age <= results[i-1].Age,
				"Results should be ordered by age DESC")
		}
	})

	suite.Run("MultipleOrderByColumns", func() {
		type Result struct {
			ID       string `bun:"id"`
			IsActive bool   `bun:"is_active"`
			Age      int16  `bun:"age"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "is_active", "age").
			OrderByExpr(func(eb orm.ExprBuilder) any {
				return eb.Order(func(ob orm.OrderBuilder) {
					ob.Column("is_active").Asc()
				})
			}).
			OrderByExpr(func(eb orm.ExprBuilder) any {
				return eb.Order(func(ob orm.OrderBuilder) {
					ob.Column("age").Desc()
				})
			}).
			Limit(10).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)
	})
}

func (suite *EBBasicExpressionsTestSuite) TestCase() {
	suite.Run("SimpleCaseExpression", func() {
		type Result struct {
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
			Category  string `bun:"category"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Case(func(cb orm.CaseBuilder) {
					cb.When(
						func(cond orm.ConditionBuilder) {
							cond.GreaterThan("view_count", 80)
						}).
						Then("Popular").
						When(func(cond orm.ConditionBuilder) {
							cond.GreaterThan("view_count", 30)
						}).
						Then("Moderate").
						Else("Low")
				})
			}, "category").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.Category)
		}
	})

	suite.Run("NestedCaseWithNullHandling", func() {
		type Result struct {
			Title         string `bun:"title"`
			Status        string `bun:"status"`
			DisplayStatus string `bun:"display_status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Coalesce(
					eb.NullIf(eb.Column("status"), "'draft'"),
					"'Working Draft'",
				)
			}, "display_status").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.DisplayStatus)
		}
	})

	suite.Run("ComplexCaseWithMultipleConditions", func() {
		type Result struct {
			Title        string `bun:"title"`
			Status       string `bun:"status"`
			ViewCount    int64  `bun:"view_count"`
			ViewCategory string `bun:"view_category"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Coalesce(
					eb.NullIf(
						eb.Case(func(cb orm.CaseBuilder) {
							cb.When(
								func(cond orm.ConditionBuilder) {
									cond.GreaterThan("view_count", 100)
								}).
								Then("High").
								When(func(cond orm.ConditionBuilder) {
									cond.GreaterThan("view_count", 50)
								}).
								Then("Medium").
								Else("Low")
						}),
						"Low",
					),
					"No Views",
				)
			}, "view_category").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ViewCategory)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestSubQuery() {
	suite.Run("SimpleSubQueryInSelect", func() {
		type Result struct {
			ID        string  `bun:"id"`
			Title     string  `bun:"title"`
			ViewCount int64   `bun:"view_count"`
			AvgViews  float64 `bun:"avg_views"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Post)(nil)).
						SelectExpr(func(eb orm.ExprBuilder) any {
							return eb.AvgColumn("view_count")
						})
				})
			}, "avg_views").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.True(result.AvgViews > 0, "Average views should be positive")
		}
	})

	suite.Run("SubQueryInWhereClause", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThanOrEqualExpr("view_count", func(eb orm.ExprBuilder) any {
					return eb.SubQuery(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.AvgColumn("view_count")
							})
					})
				})
			}).
			OrderByDesc("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
		}
	})

	suite.Run("CorrelatedSubQuery", func() {
		type Result struct {
			ID        string `bun:"id"`
			Name      string `bun:"name"`
			PostCount int64  `bun:"post_count"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Post)(nil)).
						SelectExpr(func(eb orm.ExprBuilder) any {
							return eb.CountAll()
						}).
						Where(func(cb orm.ConditionBuilder) {
							cb.EqualsColumn("p.user_id", "u.id")
						})
				})
			}, "post_count").
			OrderBy("name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
			suite.True(result.PostCount >= 0, "Post count should be non-negative")
		}
	})

	suite.Run("SubQueryWithAggregateFunction", func() {
		type Result struct {
			ID         string `bun:"id"`
			Title      string `bun:"title"`
			ViewCount  int64  `bun:"view_count"`
			MaxViews   int64  `bun:"max_views"`
			IsTopViews bool   `bun:"is_top_views"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Post)(nil)).
						SelectExpr(func(eb orm.ExprBuilder) any {
							return eb.MaxColumn("view_count")
						})
				})
			}, "max_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Case(func(cb orm.CaseBuilder) {
					cb.When(func(cond orm.ConditionBuilder) {
						cond.EqualsExpr("view_count", func(eb orm.ExprBuilder) any {
							return eb.SubQuery(func(sq orm.SelectQuery) {
								sq.Model((*Post)(nil)).
									SelectExpr(func(eb orm.ExprBuilder) any {
										return eb.MaxColumn("view_count")
									})
							})
						})
					}).
						Then(true).
						Else(false)
				})
			}, "is_top_views").
			OrderByDesc("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		var foundTopView bool
		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Title)
			suite.True(result.MaxViews > 0, "Max views should be positive")
			suite.True(result.ViewCount <= result.MaxViews,
				"View count should not exceed max views")

			if result.IsTopViews {
				foundTopView = true

				suite.Equal(result.ViewCount, result.MaxViews,
					"Top view post should have view_count equal to max_views")
			}
		}

		suite.True(foundTopView, "Should find at least one top view post")
	})
}

// TestExists tests the Exists expression.
func (suite *EBBasicExpressionsTestSuite) TestExists() {
	suite.Run("ExistsInWhereClause", func() {
		type Result struct {
			ID    string `bun:"id"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "email").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Exists(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("p.user_id", "u.id")
							})
					})
				})
			}).
			OrderBy("name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
		}
	})

	suite.Run("ExistsWithComplexCondition", func() {
		type UserWithHighViewPosts struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
		}

		var results []UserWithHighViewPosts

		err := suite.selectUsers().
			Select("id", "name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Exists(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("p.user_id", "u.id")
								cb.GreaterThan("p.view_count", 50)
							})
					})
				})
			}).
			OrderBy("name").
			Scan(suite.ctx, &results)

		suite.NoError(err)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
		}
	})

	suite.Run("ExistsWithAND", func() {
		type Result struct {
			ID    string `bun:"id"`
			Title string `bun:"title"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title").
			Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThan("view_count", 20)
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Exists(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("u.id", "p.user_id")
								cb.StartsWith("u.name", "A")
							})
					})
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)

		for _, result := range results {
			suite.NotEmpty(result.ID)
		}
	})
}

// TestNotExists tests the NotExists expression.
func (suite *EBBasicExpressionsTestSuite) TestNotExists() {
	suite.Run("NotExistsInWhereClause", func() {
		type Result struct {
			ID    string `bun:"id"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "email").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotExists(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("p.user_id", "u.id")
							})
					})
				})
			}).
			OrderBy("name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
		}
	})

	suite.Run("NotExistsWithComplexCondition", func() {
		type UserWithoutHighViewPosts struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
		}

		var results []UserWithoutHighViewPosts

		err := suite.selectUsers().
			Select("id", "name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotExists(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("p.user_id", "u.id")
								cb.GreaterThan("p.view_count", 100)
							})
					})
				})
			}).
			OrderBy("name").
			Scan(suite.ctx, &results)

		suite.NoError(err)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
		}
	})

	suite.Run("ExistsAndNotExistsCombined", func() {
		type Result struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Exists(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("p.user_id", "u.id")
							})
					})
				})
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotExists(func(sq orm.SelectQuery) {
						sq.Model((*Post)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("p.user_id", "u.id")
								cb.GreaterThan("p.view_count", 100)
							})
					})
				})
			}).
			OrderBy("name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
		}
	})
}

// TestNot tests the Not method for negating boolean expressions.
func (suite *EBBasicExpressionsTestSuite) TestNot() {
	suite.Run("NotWithEqualityCondition", func() {
		type Result struct {
			ID     string `bun:"id"`
			Title  string `bun:"title"`
			Status string `bun:"status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Not(eb.Equals(eb.Column("status"), "published"))
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEqual("published", result.Status, "Status should not be 'published'")
		}
	})

	suite.Run("NotWithComparisonCondition", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Not(eb.GreaterThan(eb.Column("view_count"), 50))
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.True(result.ViewCount <= 50, "ViewCount should be <= 50")
		}
	})

	suite.Run("NotWithNullCheck", func() {
		type Result struct {
			ID          string `bun:"id"`
			Title       string `bun:"title"`
			Description string `bun:"description"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "description").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Not(eb.IsNull(eb.Column("description")))
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.Description)
		}
	})

	suite.Run("NotInSelectExpression", func() {
		type Result struct {
			ID         string `bun:"id"`
			Status     string `bun:"status"`
			IsNotDraft bool   `bun:"is_not_draft"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Not(eb.Equals(eb.Column("status"), "draft"))
			}, "is_not_draft").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			expected := result.Status != "draft"
			suite.Equal(expected, result.IsNotDraft, "IsNotDraft should match expectation")
		}
	})
}

// TestAny tests the Any expression method.
// Note: SQLite does not support ANY/ALL operators natively and simulation is not trivial.
// MySQL supports ANY/ALL but does not allow LIMIT in ANY/ALL subqueries (Error 1235).
func (suite *EBBasicExpressionsTestSuite) TestAny() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("Test skipped for %s (ANY operator not supported)", suite.ds.Kind)
	}

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("Test skipped for %s (LIMIT in ANY/ALL subqueries not supported)", suite.ds.Kind)
	}

	suite.Run("AnyWithEqualityCondition", func() {
		type Result struct {
			ID    string `bun:"id"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "email").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Equals(
						eb.Column("id"),
						eb.Any(func(sq orm.SelectQuery) {
							sq.Model((*Post)(nil)).
								Select("user_id").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("status", "published")
								}).
								Limit(3)
						}),
					)
				})
			}).
			OrderBy("name").
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.ID)
			suite.NotEmpty(result.Name)
		}
	})

	suite.Run("AnyWithGreaterThanCondition", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.GreaterThan(
						eb.Column("view_count"),
						eb.Any(func(sq orm.SelectQuery) {
							sq.Model((*Post)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(50)
								}).
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("status", "published")
								}).
								Limit(1)
						}),
					)
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.True(result.ViewCount > 50, "ViewCount should be > 50")
		}
	})
}

// TestAll tests the All expression method.
// Note: SQLite does not support ANY/ALL operators natively and simulation is not trivial.
// MySQL supports ANY/ALL but does not allow LIMIT in ANY/ALL subqueries (Error 1235).
func (suite *EBBasicExpressionsTestSuite) TestAll() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("Test skipped for %s (ALL operator not supported)", suite.ds.Kind)
	}

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("Test skipped for %s (LIMIT in ANY/ALL subqueries not supported)", suite.ds.Kind)
	}

	suite.Run("AllWithEqualityCondition", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.GreaterThanOrEqual(
						eb.Column("view_count"),
						eb.All(func(sq orm.SelectQuery) {
							sq.Model((*Post)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(10)
								}).
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("status", "draft")
								}).
								Limit(1)
						}),
					)
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.True(result.ViewCount >= 10, "ViewCount should be >= 10")
		}
	})

	suite.Run("AllWithLessThanCondition", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.LessThan(
						eb.Column("view_count"),
						eb.All(func(sq orm.SelectQuery) {
							sq.Model((*Post)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(200)
								}).
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("status", "published")
								}).
								Limit(1)
						}),
					)
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.True(result.ViewCount < 200, "ViewCount should be < 200")
		}
	})
}

// TestArithmeticOperators tests the Add, Subtract, Multiply, Divide, and Paren methods.
func (suite *EBBasicExpressionsTestSuite) TestArithmeticOperators() {
	suite.Run("BasicArithmeticOperations", func() {
		type Result struct {
			ID         string  `bun:"id"`
			ViewCount  int64   `bun:"view_count"`
			Added      int64   `bun:"added"`
			Subtracted int64   `bun:"subtracted"`
			Multiplied int64   `bun:"multiplied"`
			Divided    float64 `bun:"divided"`
			Complex    int64   `bun:"complex"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Add(eb.Column("view_count"), 10)
			}, "added").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Subtract(eb.Column("view_count"), 5)
			}, "subtracted").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Multiply(eb.Column("view_count"), 2)
			}, "multiplied").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Divide(eb.Column("view_count"), 2)
			}, "divided").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Subtract(
					eb.Multiply(
						eb.Paren(eb.Add(eb.Column("view_count"), 10)),
						2,
					),
					5,
				)
			}, "complex").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.Equal(result.ViewCount+10, result.Added)
			suite.Equal(result.ViewCount-5, result.Subtracted)
			suite.Equal(result.ViewCount*2, result.Multiplied)
			suite.InDelta(float64(result.ViewCount)/2.0, result.Divided, 0.01)

			expected := (result.ViewCount+10)*2 - 5
			suite.Equal(expected, result.Complex)
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestExpr() {
	suite.Run("SimpleArithmeticExpression", func() {
		type Result struct {
			ID         string  `bun:"id"`
			ViewCount  int64   `bun:"view_count"`
			Doubled    int64   `bun:"doubled"`
			Multiplied int64   `bun:"multiplied"`
			Divided    float64 `bun:"divided"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? * 2", eb.Column("view_count"))
			}, "doubled").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? * ?", eb.Column("view_count"), 3)
			}, "multiplied").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 2.0", eb.Column("view_count"))
			}, "divided").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.Equal(result.ViewCount*2, result.Doubled, "Doubled should equal view_count * 2")
			suite.Equal(result.ViewCount*3, result.Multiplied, "Multiplied should equal view_count * 3")
			suite.InDelta(float64(result.ViewCount)/2.0, result.Divided, 0.1,
				"Divided should equal view_count / 2")
		}
	})

	suite.Run("StringConcatenationExpression", func() {
		type Result struct {
			ID              string `bun:"id"`
			Title           string `bun:"title"`
			Status          string `bun:"status"`
			TitleWithStatus string `bun:"title_with_status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Concat(eb.Column("title"), "' - '", eb.Column("status"))
			}, "title_with_status").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.Contains(result.TitleWithStatus, result.Title,
				"Concatenated string should contain title")
			suite.Contains(result.TitleWithStatus, result.Status,
				"Concatenated string should contain status")
		}
	})

	suite.Run("ComplexExpressionWithFunctions", func() {
		type Result struct {
			ID            string  `bun:"id"`
			ViewCount     int64   `bun:"view_count"`
			AbsDifference int64   `bun:"abs_difference"`
			RoundedAvg    float64 `bun:"rounded_avg"`
		}

		var results []Result

		avgViewCount := float64(0)
		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}).
			Scan(suite.ctx, &avgViewCount)
		suite.NoError(err)

		err = suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Abs(eb.Expr("? - ?", eb.Column("view_count"), int(avgViewCount)))
			}, "abs_difference").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Round(eb.Expr("? / 10.0", eb.Column("view_count")), 1)
			}, "rounded_avg").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.True(result.AbsDifference >= 0, "Absolute difference should be non-negative")
			suite.True(result.RoundedAvg >= 0, "Rounded average should be non-negative")
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestExprs() {
	suite.Run("CombineMultipleExpressions", func() {
		type Result struct {
			ID       string `bun:"id"`
			Name     string `bun:"name"`
			Email    string `bun:"email"`
			Combined string `bun:"combined"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "email").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Exprs(
					eb.Column("name"),
					eb.Column("email"),
					eb.Column("age"),
				)
			}, "combined").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)
	})
}

func (suite *EBBasicExpressionsTestSuite) TestExprsWithSep() {
	suite.Run("CombineConditionsWithOR", func() {
		type Result struct {
			ID        string `bun:"id"`
			Name      string `bun:"name"`
			Age       int    `bun:"age"`
			IsActive  bool   `bun:"is_active"`
			MatchesOR bool   `bun:"matches_or"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "age", "is_active").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Combine conditions: age > 28 OR is_active = false
				return eb.ExprsWithSep(
					" OR ",
					eb.Expr("age > ?", 28),
					eb.Expr("is_active = ?", false),
				)
			}, "matches_or").
			OrderBy("name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			// Verify: should be true if age > 28 OR is_active = false
			expected := result.Age > 28 || !result.IsActive
			suite.Equal(expected, result.MatchesOR, "OR condition should match expected result")
		}
	})

	suite.Run("CombineArithmeticWithAddition", func() {
		type Result struct {
			ID    string `bun:"id"`
			Name  string `bun:"name"`
			Age   int    `bun:"age"`
			Total int    `bun:"total"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Combine: age + 10 + 5
				return eb.ExprsWithSep(
					" + ",
					eb.Column("age"),
					eb.Literal(10),
					eb.Literal(5),
				)
			}, "total").
			OrderBy("name").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			// Verify: total should be age + 10 + 5
			expected := result.Age + 15
			suite.Equal(expected, result.Total, "Addition should calculate correctly")
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestExprByDialect() {
	// Test: Cross-database string concatenation
	// Different databases use different syntax for string concatenation:
	// - SQLite: uses || operator
	// - PostgreSQL/MySQL: use CONCAT function
	suite.Run("CrossDatabaseStringConcatenation", func() {
		type Result struct {
			ID       string `bun:"id"`
			Name     string `bun:"name"`
			Email    string `bun:"email"`
			FullInfo string `bun:"full_info"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "email").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Use ExprByDialect to handle different database syntaxes
				return eb.ExprByDialect(orm.DialectExprs{
					SQLite: func() schema.QueryAppender {
						// SQLite uses || for concatenation
						return eb.Expr("? || ' <' || ? || '>'",
							eb.Column("name"),
							eb.Column("email"))
					},
					Default: func() schema.QueryAppender {
						// PostgreSQL and MySQL use CONCAT
						return eb.Expr("CONCAT(?, ' <', ?, '>')",
							eb.Column("name"),
							eb.Column("email"))
					},
				})
			}, "full_info").
			OrderBy("name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.FullInfo)
			suite.Contains(result.FullInfo, result.Name, "Full info should contain name")
			suite.Contains(result.FullInfo, result.Email, "Full info should contain email")
			suite.Contains(result.FullInfo, "<", "Full info should contain opening bracket")
			suite.Contains(result.FullInfo, ">", "Full info should contain closing bracket")
		}
	})

	suite.Run("DatabaseSpecificCaseExpression", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
			Category  string `bun:"category"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// All databases support CASE expression the same way
				// But we test ExprByDialect with a common expression
				return eb.ExprByDialect(orm.DialectExprs{
					Default: func() schema.QueryAppender {
						return eb.Case(func(cb orm.CaseBuilder) {
							cb.When(
								func(cond orm.ConditionBuilder) {
									cond.GreaterThan("view_count", 80)
								}).
								Then("High").
								When(func(cond orm.ConditionBuilder) {
									cond.GreaterThan("view_count", 30)
								}).
								Then("Medium").
								Else("Low")
						})
					},
				})
			}, "category").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.NoError(err)
		suite.NotEmpty(results)

		for _, result := range results {
			suite.NotEmpty(result.Category)
			suite.Contains([]string{"High", "Medium", "Low"}, result.Category,
				"Category should be one of: High, Medium, Low")
		}
	})
}

func (suite *EBBasicExpressionsTestSuite) TestExecByDialect() {
	suite.Run("DatabaseSpecificSideEffects", func() {
		// Track which database callback was executed
		var executed string

		// Create a simple query to test with
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)

		// Get the expression builder from the query
		eb := query.ExprBuilder()

		// Execute database-specific callbacks
		eb.ExecByDialect(orm.DialectExecs{
			Postgres: func() {
				executed = "postgres"
			},
			MySQL: func() {
				executed = "mysql"
			},
			SQLite: func() {
				executed = "sqlite"
			},
		})

		// Verify the correct callback was executed based on current database
		switch suite.ds.Kind {
		case config.Postgres:
			suite.Equal("postgres", executed, "Postgres callback should be executed")
		case config.MySQL:
			suite.Equal("mysql", executed, "MySQL callback should be executed")
		case config.SQLite:
			suite.Equal("sqlite", executed, "SQLite callback should be executed")
		}
	})

	suite.Run("DefaultCallback", func() {
		var executed string

		// Create a simple query to test with
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Only provide default callback
		eb.ExecByDialect(orm.DialectExecs{
			Default: func() {
				executed = "default"
			},
		})

		// Default callback should be executed for all databases
		suite.Equal("default", executed, "Default callback should be executed")
	})
}

func (suite *EBBasicExpressionsTestSuite) TestExecByDialectWithErr() {
	suite.Run("DatabaseSpecificWithError", func() {
		var executed string

		// Create a simple query to test with
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Execute database-specific callbacks that return no error
		err := eb.ExecByDialectWithErr(orm.DialectExecsWithErr{
			Postgres: func() error {
				executed = "postgres"

				return nil
			},
			MySQL: func() error {
				executed = "mysql"

				return nil
			},
			SQLite: func() error {
				executed = "sqlite"

				return nil
			},
		})

		suite.NoError(err)

		// Verify the correct callback was executed
		switch suite.ds.Kind {
		case config.Postgres:
			suite.Equal("postgres", executed, "Postgres callback should be executed")
		case config.MySQL:
			suite.Equal("mysql", executed, "MySQL callback should be executed")
		case config.SQLite:
			suite.Equal("sqlite", executed, "SQLite callback should be executed")
		}
	})

	suite.Run("ErrorHandling", func() {
		// Create a simple query to test with
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Execute callbacks that return an error
		testErr := fmt.Errorf("test error from %s", suite.ds.Kind)
		err := eb.ExecByDialectWithErr(orm.DialectExecsWithErr{
			Postgres: func() error {
				return testErr
			},
			MySQL: func() error {
				return testErr
			},
			SQLite: func() error {
				return testErr
			},
		})

		suite.Error(err, "ExecByDialectWithErr should return error when callback fails")
		suite.Equal(testErr, err, "Should return the exact error from callback")
	})

	suite.Run("DefaultCallbackWithError", func() {
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Test successful default callback
		err := eb.ExecByDialectWithErr(orm.DialectExecsWithErr{
			Default: func() error {
				return nil
			},
		})
		suite.NoError(err)

		// Test default callback returning error
		testErr := errors.New("default callback error")
		err = eb.ExecByDialectWithErr(orm.DialectExecsWithErr{
			Default: func() error {
				return testErr
			},
		})
		suite.Error(err, "Default callback error should be returned")
		suite.Equal(testErr, err, "Should return the exact error from default callback")
	})
}

func (suite *EBBasicExpressionsTestSuite) TestFragmentByDialect() {
	suite.Run("DatabaseSpecificFragments", func() {
		// Create a simple query to test with
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Generate database-specific fragments
		fragment, err := eb.FragmentByDialect(orm.DialectFragments{
			Postgres: func() ([]byte, error) {
				return []byte("/* PostgreSQL fragment */"), nil
			},
			MySQL: func() ([]byte, error) {
				return []byte("/* MySQL fragment */"), nil
			},
			SQLite: func() ([]byte, error) {
				return []byte("/* SQLite fragment */"), nil
			},
		})

		suite.NoError(err)
		suite.NotNil(fragment)

		// Verify the correct fragment was generated
		fragmentStr := string(fragment)
		switch suite.ds.Kind {
		case config.Postgres:
			suite.Contains(fragmentStr, "PostgreSQL", "Should contain PostgreSQL fragment")
		case config.MySQL:
			suite.Contains(fragmentStr, "MySQL", "Should contain MySQL fragment")
		case config.SQLite:
			suite.Contains(fragmentStr, "SQLite", "Should contain SQLite fragment")
		}
	})

	suite.Run("FragmentErrorHandling", func() {
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Generate fragment that returns an error
		testErr := errors.New("fragment generation error")
		fragment, err := eb.FragmentByDialect(orm.DialectFragments{
			Postgres: func() ([]byte, error) {
				return nil, testErr
			},
			MySQL: func() ([]byte, error) {
				return nil, testErr
			},
			SQLite: func() ([]byte, error) {
				return nil, testErr
			},
		})

		suite.Error(err, "FragmentByDialect should return error when callback fails")
		suite.Equal(testErr, err, "Should return the exact error from callback")
		suite.Nil(fragment)
	})

	suite.Run("DefaultFragmentCallback", func() {
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Use default callback
		defaultFragment := []byte("/* Default fragment */")
		fragment, err := eb.FragmentByDialect(orm.DialectFragments{
			Default: func() ([]byte, error) {
				return defaultFragment, nil
			},
		})

		suite.NoError(err)
		suite.Equal(defaultFragment, fragment, "Should return default fragment")
	})

	suite.Run("EmptyFragment", func() {
		query := suite.db.NewSelect().Model((*Post)(nil)).Limit(1)
		eb := query.ExprBuilder()

		// Return empty/nil fragment
		fragment, err := eb.FragmentByDialect(orm.DialectFragments{
			Default: func() ([]byte, error) {
				return nil, nil
			},
		})

		suite.NoError(err)
		suite.Nil(fragment)
	})
}
