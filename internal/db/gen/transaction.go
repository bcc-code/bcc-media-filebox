package db

import (
	"context"
	"database/sql"
	"errors"
)

// BeginTx exposes a transaction without leaking Queries' generated DBTX field
// to callers. sqlc leaves hand-written files in this package untouched when it
// regenerates the query methods.
func (q *Queries) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, *Queries, error) {
	beginner, ok := q.db.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return nil, nil, errors.New("database connection does not support transactions")
	}
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	return tx, q.WithTx(tx), nil
}
