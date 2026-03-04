package sequence

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const dbStoreTableName = "sys_sequence_rule"

// RuleModel is the internal ORM model for the sys_sequence_rule table.
type RuleModel struct {
	orm.BaseModel `bun:"table:sys_sequence_rule,alias:ssr"`
	orm.Model

	Key              string          `bun:",notnull,unique"`
	Name             string          `bun:",notnull"`
	Prefix           *string         `bun:",type:varchar(32)"`
	Suffix           *string         `bun:",type:varchar(32)"`
	DateFormat       *string         `bun:",type:varchar(32)"`
	SeqLength        int16           `bun:",notnull,default:4"`
	SeqStep          int16           `bun:",notnull,default:1"`
	StartValue       int             `bun:",notnull,default:0"`
	MaxValue         int             `bun:",notnull,default:0"`
	OverflowStrategy string          `bun:",notnull,default:'error'"`
	ResetCycle       string          `bun:",notnull,default:'N'"`
	CurrentValue     int             `bun:",notnull,default:0"`
	LastResetAt      *timex.DateTime `bun:",type:timestamp"`
	IsActive         bool            `bun:",notnull,default:true"`
	Remark           *string         `bun:",type:varchar(256)"`
}

// toRule converts the ORM model to the public Rule type.
func (m *RuleModel) toRule() *Rule {
	rule := &Rule{
		Key:              m.Key,
		Name:             m.Name,
		SeqLength:        int(m.SeqLength),
		SeqStep:          int(m.SeqStep),
		StartValue:       m.StartValue,
		MaxValue:         m.MaxValue,
		OverflowStrategy: OverflowStrategy(m.OverflowStrategy),
		ResetCycle:       ResetCycle(m.ResetCycle),
		CurrentValue:     m.CurrentValue,
		LastResetAt:      m.LastResetAt,
		IsActive:         m.IsActive,
	}

	if m.Prefix != nil {
		rule.Prefix = *m.Prefix
	}

	if m.Suffix != nil {
		rule.Suffix = *m.Suffix
	}

	if m.DateFormat != nil {
		rule.DateFormat = *m.DateFormat
	}

	return rule
}

// DBStore implements Store using a relational database.
// Table name is fixed to sys_sequence_rule.
type DBStore struct {
	db orm.DB
}

// NewDBStore creates a new database-backed sequence store.
func NewDBStore(db orm.DB) Store {
	return &DBStore{db: db}
}

// Init creates the sys_sequence_rule table if it does not exist.
// Implements contract.Initializer.
func (s *DBStore) Init(ctx context.Context) error {
	_, err := s.db.NewCreateTable().
		Model((*RuleModel)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", dbStoreTableName, err)
	}

	logger.Infof("Table %s ensured", dbStoreTableName)

	return nil
}

func (s *DBStore) Load(ctx context.Context, key string) (*Rule, error) {
	var model RuleModel

	err := s.db.NewSelect().
		Model(&model).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("key", key).
				Equals("is_active", true)
		}).
		Scan(ctx)
	if err != nil {
		return nil, ErrRuleNotFound
	}

	return model.toRule(), nil
}

func (s *DBStore) Increment(ctx context.Context, key string, step int, count int, startValue int, resetNeeded bool) (int, error) {
	var newValue int

	err := s.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		var model RuleModel

		if err := tx.NewSelect().
			Model(&model).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("key", key)
			}).
			ForUpdate().
			Scan(txCtx); err != nil {
			return ErrRuleNotFound
		}

		if resetNeeded {
			model.CurrentValue = startValue
			now := timex.Now()
			model.LastResetAt = &now
		}

		model.CurrentValue += step * count
		newValue = model.CurrentValue

		if _, err := tx.NewUpdate().
			Model((*RuleModel)(nil)).
			Set("current_value", newValue).
			Set("last_reset_at", model.LastResetAt).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", model.ID)
			}).
			Exec(txCtx); err != nil {
			return fmt.Errorf("failed to update sequence rule: %w", err)
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return newValue, nil
}
