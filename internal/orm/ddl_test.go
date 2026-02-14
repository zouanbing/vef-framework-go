package orm_test

import (
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *OrmTestSuite) suite.TestingSuite {
		return &DDLTestSuite{OrmTestSuite: base}
	})
}

// DDLModel is a dedicated test model for DDL operations.
type DDLModel struct {
	bun.BaseModel `bun:"table:test_ddl_model,alias:dm"`
	orm.IDModel

	Label string `json:"label" bun:"label,notnull"`
	Score int    `json:"score" bun:"score,notnull,default:0"`
}

// DDLTestSuite tests DDL operations including CreateTable, DropTable, CreateIndex,
// DropIndex, TruncateTable, AddColumn, and DropColumn across all databases.
type DDLTestSuite struct {
	*OrmTestSuite
}

func (suite *DDLTestSuite) SetupSuite() {
	db := suite.getBunDB()
	db.RegisterModel((*DDLModel)(nil))
}

// TestCreateTableAndDropTable tests creating and dropping a table via the orm.DB interface.
func (suite *DDLTestSuite) TestCreateTableAndDropTable() {
	suite.Run("CreateTable_IfNotExists", func() {
		_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
		suite.NoError(err, "Should drop table if it exists")

		_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).IfNotExists().Exec(suite.ctx)
		suite.NoError(err, "Should create table via orm.DB interface")

		// Verify by inserting a row
		_, err = suite.db.NewInsert().
			Model(&DDLModel{Label: "hello", Score: 42}).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert into newly created table")
	})

	suite.Run("CreateTable_Idempotent", func() {
		_, err := suite.db.NewCreateTable().Model((*DDLModel)(nil)).IfNotExists().Exec(suite.ctx)
		suite.NoError(err, "IfNotExists should make CreateTable idempotent")
	})

	suite.Run("DropTable_IfExists", func() {
		_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
		suite.NoError(err, "Should drop existing table")

		// Drop again — should not fail because of IfExists
		_, err = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
		suite.NoError(err, "IfExists should make DropTable idempotent")
	})
}

// TestCreateIndexAndDropIndex tests creating and dropping indexes via the orm.DB interface.
func (suite *DDLTestSuite) TestCreateIndexAndDropIndex() {
	// Setup: ensure the table exists
	_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	suite.Require().NoError(err)
	_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.Require().NoError(err)

	defer func() {
		_, _ = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	}()

	suite.Run("CreateIndex", func() {
		_, err := suite.db.NewCreateIndex().
			Model((*DDLModel)(nil)).
			Index("idx_ddl_model_label").
			Column("label").
			Exec(suite.ctx)
		suite.NoError(err, "Should create index via orm.DB interface")
	})

	suite.Run("CreateUniqueIndex", func() {
		_, err := suite.db.NewCreateIndex().
			Model((*DDLModel)(nil)).
			Index("idx_ddl_model_label_unique").
			Column("label").
			Unique().
			Exec(suite.ctx)
		suite.NoError(err, "Should create unique index")
	})

	suite.Run("DropIndex", func() {
		// MySQL requires table context for DROP INDEX and does not support IF EXISTS.
		// PostgreSQL and SQLite support standalone DROP INDEX IF EXISTS.
		if suite.dbKind == config.MySQL {
			_, err := suite.db.NewRaw("ALTER TABLE test_ddl_model DROP INDEX idx_ddl_model_label").
				Exec(suite.ctx)
			suite.NoError(err, "Should drop index on MySQL via raw SQL")
		} else {
			_, err := suite.db.NewDropIndex().
				Index("idx_ddl_model_label").
				IfExists().
				Exec(suite.ctx)
			suite.NoError(err, "Should drop index via orm.DB interface")
		}
	})
}

// TestTruncateTable tests truncating a table via the orm.DB interface.
func (suite *DDLTestSuite) TestTruncateTable() {
	// Setup: ensure the table exists and has data
	_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	suite.Require().NoError(err)
	_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.Require().NoError(err)

	defer func() {
		_, _ = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	}()

	models := []DDLModel{
		{Label: "a", Score: 1},
		{Label: "b", Score: 2},
		{Label: "c", Score: 3},
	}
	_, err = suite.db.NewInsert().Model(&models).Exec(suite.ctx)
	suite.Require().NoError(err)

	// Verify data exists
	count, err := suite.db.NewSelect().Model((*DDLModel)(nil)).Count(suite.ctx)
	suite.Require().NoError(err)
	suite.Equal(int64(3), count, "Should have 3 rows before truncate")

	// Truncate
	_, err = suite.db.NewTruncateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.NoError(err, "Should truncate table via orm.DB interface")

	// Verify data removed
	count, err = suite.db.NewSelect().Model((*DDLModel)(nil)).Count(suite.ctx)
	suite.NoError(err)
	suite.Equal(int64(0), count, "Should have 0 rows after truncate")
}

// TestAddColumnAndDropColumn tests adding and dropping columns via the orm.DB interface.
func (suite *DDLTestSuite) TestAddColumnAndDropColumn() {
	// Setup: ensure the table exists
	_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	suite.Require().NoError(err)
	_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.Require().NoError(err)

	defer func() {
		_, _ = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	}()

	suite.Run("AddColumn", func() {
		_, err := suite.db.NewAddColumn().
			Table("test_ddl_model").
			ColumnExpr("extra_field VARCHAR(100)").
			Exec(suite.ctx)
		suite.NoError(err, "Should add column via orm.DB interface")
	})

	suite.Run("DropColumn", func() {
		_, err := suite.db.NewDropColumn().
			Table("test_ddl_model").
			Column("extra_field").
			Exec(suite.ctx)
		suite.NoError(err, "Should drop column via orm.DB interface")
	})
}

// TestFluentChaining verifies that DDL queries support fluent method chaining.
func (suite *DDLTestSuite) TestFluentChaining() {
	suite.Run("CreateTableQuery_Chaining", func() {
		q := suite.db.NewCreateTable().
			Model((*DDLModel)(nil)).
			IfNotExists().
			Temp()
		suite.NotNil(q, "CreateTableQuery should support fluent chaining")
	})

	suite.Run("DropTableQuery_Chaining", func() {
		q := suite.db.NewDropTable().
			Model((*DDLModel)(nil)).
			IfExists()
		suite.NotNil(q, "DropTableQuery should support fluent chaining")
	})

	suite.Run("CreateIndexQuery_Chaining", func() {
		q := suite.db.NewCreateIndex().
			Model((*DDLModel)(nil)).
			Index("test_idx").
			Column("label").
			Unique().
			IfNotExists()
		suite.NotNil(q, "CreateIndexQuery should support fluent chaining")
	})

	suite.Run("DropIndexQuery_Chaining", func() {
		q := suite.db.NewDropIndex().
			Index("test_idx").
			IfExists()
		suite.NotNil(q, "DropIndexQuery should support fluent chaining")
	})

	suite.Run("TruncateTableQuery_Chaining", func() {
		q := suite.db.NewTruncateTable().
			Model((*DDLModel)(nil))
		suite.NotNil(q, "TruncateTableQuery should support fluent chaining")
	})

	suite.Run("AddColumnQuery_Chaining", func() {
		q := suite.db.NewAddColumn().
			Table("test_ddl_model").
			ColumnExpr("extra VARCHAR(50)")
		suite.NotNil(q, "AddColumnQuery should support fluent chaining")
	})

	suite.Run("DropColumnQuery_Chaining", func() {
		q := suite.db.NewDropColumn().
			Table("test_ddl_model").
			Column("extra")
		suite.NotNil(q, "DropColumnQuery should support fluent chaining")
	})
}

// TestCreateTableString tests String() output for CreateTable and DropTable.
func (suite *DDLTestSuite) TestCreateTableString() {
	sql := suite.db.NewCreateTable().
		Model((*DDLModel)(nil)).
		IfNotExists().
		String()
	suite.Contains(sql, "CREATE TABLE", "String() should contain CREATE TABLE")
	suite.Contains(sql, "IF NOT EXISTS", "String() should contain IF NOT EXISTS")
}

// TestDropTableString tests String() output for DropTable.
func (suite *DDLTestSuite) TestDropTableString() {
	sql := suite.db.NewDropTable().
		Model((*DDLModel)(nil)).
		IfExists().
		String()
	suite.Contains(sql, "DROP TABLE", "String() should contain DROP TABLE")
	suite.Contains(sql, "IF EXISTS", "String() should contain IF EXISTS")
}
