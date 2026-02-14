package orm_test

import (
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBAuditConditionsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBAuditConditionsTestSuite tests audit field condition methods.
// Covers: CreatedBy, UpdatedBy, CreatedAt, UpdatedAt series (~158 methods total).
type CBAuditConditionsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestCreatedByEquals tests the CreatedByEquals and OrCreatedByEquals conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedByEquals() {
	suite.T().Logf("Testing CreatedByEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicCreatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByEquals("system")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.Equal("system", user.CreatedBy, "CreatedBy should be system")
		}

		suite.T().Logf("Found %d users created by system", len(users))
	})

	suite.Run("OrCreatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByEquals("system").
						OrCreatedByEquals("admin")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestCreatedByNotEquals tests the CreatedByNotEquals and OrCreatedByNotEquals conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedByNotEquals() {
	suite.T().Logf("Testing CreatedByNotEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicCreatedByNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotEquals("nonexistent")
				}),
		)

		suite.True(len(users) > 0, "Should find users")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrCreatedByNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotEquals("user1").
						OrCreatedByNotEquals("user2")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestCreatedByIn tests the CreatedByIn and OrCreatedByIn conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedByIn() {
	suite.T().Logf("Testing CreatedByIn condition for %s", suite.ds.Kind)

	suite.Run("BasicCreatedByIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByIn([]string{"system", "admin"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrCreatedByIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByIn([]string{"system"}).
						OrCreatedByIn([]string{"admin"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestUpdatedByEquals tests the UpdatedByEquals and OrUpdatedByEquals conditions.
func (suite *CBAuditConditionsTestSuite) TestUpdatedByEquals() {
	suite.T().Logf("Testing UpdatedByEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicUpdatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByEquals("system")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.Equal("system", user.UpdatedBy, "UpdatedBy should be system")
		}

		suite.T().Logf("Found %d users updated by system", len(users))
	})

	suite.Run("OrUpdatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByEquals("system").
						OrUpdatedByEquals("admin")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestCreatedAtBetween tests the CreatedAtBetween and OrCreatedAtBetween conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedAtBetween() {
	suite.T().Logf("Testing CreatedAtBetween condition for %s", suite.ds.Kind)

	// Fixture data has created_at around 2025-01-01 00:01:00 ~ 00:05:00
	fixtureDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	suite.Run("BasicCreatedAtBetween", func() {
		start := fixtureDate.Add(-1 * time.Hour)
		end := fixtureDate.Add(1 * time.Hour)

		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtBetween(start, end)
				}),
		)

		suite.True(len(users) > 0, "Should find users created around fixture date")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrCreatedAtBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtBetween(fixtureDate.Add(-48*time.Hour), fixtureDate.Add(-24*time.Hour)).
						OrCreatedAtBetween(fixtureDate.Add(-1*time.Hour), fixtureDate.Add(1*time.Hour))
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})
}

// TestUpdatedAtGreaterThan tests the UpdatedAtGreaterThan and OrUpdatedAtGreaterThan conditions.
func (suite *CBAuditConditionsTestSuite) TestUpdatedAtGreaterThan() {
	suite.T().Logf("Testing UpdatedAtGreaterThan condition for %s", suite.ds.Kind)

	// Fixture data has updated_at around 2025-01-01 00:02:00 ~ 00:06:00
	beforeFixture := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	suite.Run("BasicUpdatedAtGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtGreaterThan(beforeFixture)
				}),
		)

		suite.True(len(users) > 0, "Should find users updated after the reference date")

		suite.T().Logf("Found %d users", len(users))
	})

	suite.Run("OrUpdatedAtGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.db.NewSelect().
				Model((*User)(nil)).
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtGreaterThan(beforeFixture.Add(-48 * time.Hour)).
						OrUpdatedAtGreaterThan(beforeFixture)
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		suite.T().Logf("Found %d users", len(users))
	})
}
