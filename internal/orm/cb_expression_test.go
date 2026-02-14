package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *OrmTestSuite) suite.TestingSuite {
		return &CBExpressionOperationsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{OrmTestSuite: base},
		}
	})
}

// CBExpressionOperationsTestSuite tests expression-based condition methods.
// Covers: EqualsExpr, NotEqualsExpr, GreaterThanExpr, LessThanExpr, Expr, and their Or variants.
type CBExpressionOperationsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestEqualsExpr tests the EqualsExpr and OrEqualsExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestEqualsExpr() {
	suite.T().Logf("Testing EqualsExpr condition for %s", suite.dbKind)

	suite.Run("BasicEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.Equal(int16(30), users[0].Age)

		suite.T().Logf("Found user: %s", users[0].Name)
	})

	suite.Run("OrEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 25)
					}).OrEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 35)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotEqualsExpr tests the NotEqualsExpr and OrNotEqualsExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestNotEqualsExpr() {
	suite.T().Logf("Testing NotEqualsExpr condition for %s", suite.dbKind)

	suite.Run("BasicNotEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")

		for _, user := range users {
			suite.NotEqual(int16(30), user.Age)
		}

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 25)
					}).OrNotEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestGreaterThanExpr tests the GreaterThanExpr and OrGreaterThanExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestGreaterThanExpr() {
	suite.T().Logf("Testing GreaterThanExpr condition for %s", suite.dbKind)

	suite.Run("BasicGreaterThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.True(users[0].Age > 30, "Age should be greater than 30")

		suite.T().Logf("Found user: %s (age: %d)", users[0].Name, users[0].Age)
	})

	suite.Run("OrGreaterThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					}).OrGreaterThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 24)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestLessThanExpr tests the LessThanExpr and OrLessThanExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestLessThanExpr() {
	suite.T().Logf("Testing LessThanExpr condition for %s", suite.dbKind)

	suite.Run("BasicLessThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.True(users[0].Age < 30, "Age should be less than 30")

		suite.T().Logf("Found user: %s (age: %d)", users[0].Name, users[0].Age)
	})

	suite.Run("OrLessThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 26)
					}).OrLessThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 31)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestExpr tests the Expr and OrExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestExpr() {
	suite.T().Logf("Testing Expr condition for %s", suite.dbKind)

	suite.Run("BasicExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age > ?", 30)
					})
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.True(users[0].Age > 30, "Age should be greater than 30")

		suite.T().Logf("Found user: %s", users[0].Name)
	})

	suite.Run("OrExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age = ?", 25)
					}).OrExpr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age = ?", 35)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("ComplexExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age BETWEEN ? AND ?", 25, 35)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}
