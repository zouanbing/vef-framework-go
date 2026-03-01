package security

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/ilxqx/vef-framework-go/orm"
)

// NonceRecord is the database model for nonce storage.
type NonceRecord struct {
	bun.BaseModel `bun:"table:security_nonces"`

	AppID     string    `bun:"app_id,pk"`
	Nonce     string    `bun:"nonce,pk"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
}

// DBNonceStore implements NonceStore using a database for persistent storage.
type DBNonceStore struct {
	db orm.DB
}

// NewDBNonceStore creates a new database-backed nonce store.
// It creates the table if it doesn't exist.
func NewDBNonceStore(ctx context.Context, db orm.DB) (NonceStore, error) {
	if _, err := db.NewCreateTable().Model((*NonceRecord)(nil)).IfNotExists().Exec(ctx); err != nil {
		return nil, err
	}
	return &DBNonceStore{db: db}, nil
}

func (s *DBNonceStore) Exists(ctx context.Context, appID, nonce string) (bool, error) {
	return s.db.NewSelect().
		Model((*NonceRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("app_id", appID).
				Equals("nonce", nonce).
				GreaterThan("expires_at", time.Now())
		}).
		Exists(ctx)
}

func (s *DBNonceStore) Store(ctx context.Context, appID, nonce string, ttl time.Duration) error {
	record := &NonceRecord{
		AppID:     appID,
		Nonce:     nonce,
		ExpiresAt: time.Now().Add(ttl),
	}
	_, err := s.db.NewInsert().
		Model(record).
		OnConflict(func(cb orm.ConflictBuilder) {
			cb.DoNothing()
		}).
		Exec(ctx)
	return err
}
