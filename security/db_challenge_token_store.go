package security

import (
	"context"
	"encoding/json"
	"time"

	"github.com/uptrace/bun"

	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
)

// ChallengeRecord is the database model for challenge token storage.
type ChallengeRecord struct {
	bun.BaseModel `bun:"table:security_challenges"`

	Token     string    `bun:"token,pk"`
	UserID    string    `bun:"user_id,notnull"`
	UserName  string    `bun:"user_name,notnull"`
	Roles     []string  `bun:"roles,type:json,nullzero"`
	Details   string    `bun:"details,type:text,nullzero"`
	Pending   []string  `bun:"pending,type:json,nullzero"`
	Resolved  []string  `bun:"resolved,type:json,nullzero"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
}

// DBChallengeTokenStore implements ChallengeTokenStore using a database for persistent storage.
type DBChallengeTokenStore struct {
	db orm.DB
}

// NewDBChallengeTokenStore creates a new database-backed challenge token store.
// It creates the table if it doesn't exist.
func NewDBChallengeTokenStore(ctx context.Context, db orm.DB) (ChallengeTokenStore, error) {
	if _, err := db.NewCreateTable().Model((*ChallengeRecord)(nil)).IfNotExists().Exec(ctx); err != nil {
		return nil, err
	}
	return &DBChallengeTokenStore{db: db}, nil
}

func (s *DBChallengeTokenStore) Generate(principal *Principal, pending, resolved []string) (string, error) {
	token := id.GenerateUUID()

	var details string
	if principal.Details != nil {
		data, err := json.Marshal(principal.Details)
		if err != nil {
			return "", err
		}
		details = string(data)
	}

	record := &ChallengeRecord{
		Token:     token,
		UserID:    principal.ID,
		UserName:  principal.Name,
		Roles:     principal.Roles,
		Details:   details,
		Pending:   pending,
		Resolved:  resolved,
		ExpiresAt: time.Now().Add(ChallengeTokenExpires),
	}

	if _, err := s.db.NewInsert().Model(record).Exec(context.Background()); err != nil {
		return "", err
	}

	return token, nil
}

func (s *DBChallengeTokenStore) Parse(token string) (*ChallengeState, error) {
	if token == "" {
		return nil, result.ErrTokenInvalid
	}

	record := new(ChallengeRecord)
	err := s.db.NewSelect().
		Model(record).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("token", token).
				GreaterThan("expires_at", time.Now())
		}).
		Scan(context.Background())
	if err != nil {
		return nil, result.ErrTokenInvalid
	}

	principal := NewUser(record.UserID, record.UserName, record.Roles...)
	if record.Details != "" {
		var details map[string]any
		if err := json.Unmarshal([]byte(record.Details), &details); err == nil {
			principal.AttemptUnmarshalDetails(details)
		}
	}

	return &ChallengeState{
		Principal: principal,
		Pending:   record.Pending,
		Resolved:  record.Resolved,
	}, nil
}
