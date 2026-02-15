package orm_test

import "github.com/ilxqx/vef-framework-go/internal/orm"

// ConditionBuilderTestSuite provides common helpers for all condition builder tests.
type ConditionBuilderTestSuite struct {
	*BaseTestSuite
}

// assertQueryReturnsUsers executes a query and returns users, scoped to fixture data.
func (suite *ConditionBuilderTestSuite) assertQueryReturnsUsers(query orm.SelectQuery) []User {
	var users []User

	err := query.Where(fixtureScope).Scan(suite.ctx, &users)
	suite.NoError(err, "Should execute query successfully")

	return users
}

// assertQueryReturnsPosts executes a query and returns posts, scoped to fixture data.
func (suite *ConditionBuilderTestSuite) assertQueryReturnsPosts(query orm.SelectQuery) []Post {
	var posts []Post

	err := query.Where(fixtureScope).Scan(suite.ctx, &posts)
	suite.NoError(err, "Should execute query successfully")

	return posts
}
