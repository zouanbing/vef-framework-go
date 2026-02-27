package orm

import "context"

type txContextKey struct{}

// ContextWithDB returns a new context with the given DB stored in it.
func ContextWithDB(ctx context.Context, db DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, db)
}

// DBFromContext extracts the DB from the context.
// Returns the DB and true if found, or nil and false if not present.
func DBFromContext(ctx context.Context) (DB, bool) {
	db, ok := ctx.Value(txContextKey{}).(DB)
	return db, ok
}
