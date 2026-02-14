package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBNotStringTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBNotStringTestSuite tests negated string operation condition methods.
// Covers: NotContains, NotStartsWith, NotEndsWith and their Any/IgnoreCase/AnyIgnoreCase variants.
type CBNotStringTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestNotContains tests NotContains and OrNotContains conditions.
func (suite *CBNotStringTestSuite) TestNotContains() {
	suite.T().Logf("Testing NotContains condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContains("name", "Alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")

		for _, user := range users {
			suite.NotContains(user.Name, "Alice", "Name should not contain Alice")
		}

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContains("name", "Alice").
						OrNotContains("name", "Bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotContainsAny tests NotContainsAny and OrNotContainsAny conditions.
func (suite *CBNotStringTestSuite) TestNotContainsAny() {
	suite.T().Logf("Testing NotContainsAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContainsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotContainsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotContainsAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotContainsIgnoreCase tests NotContainsIgnoreCase and OrNotContainsIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotContainsIgnoreCase() {
	suite.T().Logf("Testing NotContainsIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContainsIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsIgnoreCase("name", "alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice (case-insensitive)")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotContainsIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsIgnoreCase("name", "alice").
						OrNotContainsIgnoreCase("name", "bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotContainsAnyIgnoreCase tests NotContainsAnyIgnoreCase and OrNotContainsAnyIgnoreCase.
func (suite *CBNotStringTestSuite) TestNotContainsAnyIgnoreCase() {
	suite.T().Logf("Testing NotContainsAnyIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestContainsAnyIgnoreCase tests ContainsAnyIgnoreCase and OrContainsAnyIgnoreCase.
func (suite *CBNotStringTestSuite) TestContainsAnyIgnoreCase() {
	suite.T().Logf("Testing ContainsAnyIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find Alice and Bob (case-insensitive)")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotStartsWith tests NotStartsWith and OrNotStartsWith conditions.
func (suite *CBNotStringTestSuite) TestNotStartsWith() {
	suite.T().Logf("Testing NotStartsWith condition for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWith("name", "Alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWith("name", "Alice").
						OrNotStartsWith("name", "Bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotStartsWithAny tests NotStartsWithAny and OrNotStartsWithAny conditions.
func (suite *CBNotStringTestSuite) TestNotStartsWithAny() {
	suite.T().Logf("Testing NotStartsWithAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotStartsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotStartsWithAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotStartsWithIgnoreCase tests NotStartsWithIgnoreCase and OrNotStartsWithIgnoreCase.
func (suite *CBNotStringTestSuite) TestNotStartsWithIgnoreCase() {
	suite.T().Logf("Testing NotStartsWithIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWithIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithIgnoreCase("name", "alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotStartsWithIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithIgnoreCase("name", "alice").
						OrNotStartsWithIgnoreCase("name", "bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotStartsWithAnyIgnoreCase tests the full chain.
func (suite *CBNotStringTestSuite) TestNotStartsWithAnyIgnoreCase() {
	suite.T().Logf("Testing NotStartsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotStartsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotStartsWithAnyIgnoreCase("name", []string{"alice"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestStartsWithAnyIgnoreCase tests StartsWithAnyIgnoreCase and OrStartsWithAnyIgnoreCase.
func (suite *CBNotStringTestSuite) TestStartsWithAnyIgnoreCase() {
	suite.T().Logf("Testing StartsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicStartsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWithAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find Alice and Bob")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrStartsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrStartsWithAnyIgnoreCase("name", []string{"alice"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotEndsWith tests NotEndsWith and OrNotEndsWith conditions.
func (suite *CBNotStringTestSuite) TestNotEndsWith() {
	suite.T().Logf("Testing NotEndsWith condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWith("name", "Johnson")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Johnson")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWith("name", "Johnson").
						OrNotEndsWith("name", "Smith")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotEndsWithAny tests NotEndsWithAny and OrNotEndsWithAny conditions.
func (suite *CBNotStringTestSuite) TestNotEndsWithAny() {
	suite.T().Logf("Testing NotEndsWithAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithAny("name", []string{"Johnson", "Smith"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotEndsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotEndsWithAny("name", []string{"Johnson", "Smith"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotEndsWithIgnoreCase tests NotEndsWithIgnoreCase and OrNotEndsWithIgnoreCase.
func (suite *CBNotStringTestSuite) TestNotEndsWithIgnoreCase() {
	suite.T().Logf("Testing NotEndsWithIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWithIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithIgnoreCase("name", "johnson")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Johnson")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotEndsWithIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithIgnoreCase("name", "johnson").
						OrNotEndsWithIgnoreCase("name", "smith")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestNotEndsWithAnyIgnoreCase tests full chain.
func (suite *CBNotStringTestSuite) TestNotEndsWithAnyIgnoreCase() {
	suite.T().Logf("Testing NotEndsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithAnyIgnoreCase("name", []string{"johnson", "smith"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrNotEndsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotEndsWithAnyIgnoreCase("name", []string{"johnson"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestEndsWithAnyIgnoreCase tests EndsWithAnyIgnoreCase and OrEndsWithAnyIgnoreCase.
func (suite *CBNotStringTestSuite) TestEndsWithAnyIgnoreCase() {
	suite.T().Logf("Testing EndsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicEndsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWithAnyIgnoreCase("name", []string{"johnson", "smith"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find Johnson and Smith")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrEndsWithAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrEndsWithAnyIgnoreCase("name", []string{"johnson"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})
}
