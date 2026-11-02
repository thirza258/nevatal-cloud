// Package repository is the persistence layer. Each aggregate has an
// interface (what the use cases need) and a GORM implementation. Repositories
// contain no business rules: they translate method calls into queries,
// including preloads, ordering and row locks.
//
// Every method takes a context.Context first. When the context carries a
// transaction started by Transactor.WithinTransaction, the method runs inside
// it; otherwise it runs on the shared connection pool bound to the context.
package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ErrNotFound is returned when a lookup matches no row. Its text is identical
// to gorm.ErrRecordNotFound because some endpoints echo the error message to
// API clients.
var ErrNotFound = errors.New("record not found")

// ErrDuplicate is returned by methods documented to report a unique-index
// violation (SQLSTATE 23505) instead of the driver error.
var ErrDuplicate = errors.New("duplicate key")

// Transactor runs a unit of work inside a single database transaction.
type Transactor interface {
	// WithinTransaction calls fn with a context that carries a transaction.
	// Repository calls made with that context join the transaction. The
	// transaction commits when fn returns nil and rolls back otherwise. If ctx
	// already carries a transaction, fn joins it instead of starting a new one.
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type txKey struct{}

type gormTransactor struct {
	db *gorm.DB
}

// NewTransactor returns a Transactor backed by db.
func NewTransactor(db *gorm.DB) Transactor {
	return &gormTransactor{db: db}
}

func (t *gormTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// base is embedded by every GORM repository.
type base struct {
	db *gorm.DB
}

// conn returns the transaction carried by ctx, or the pool bound to ctx.
func (b base) conn(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return b.db.WithContext(ctx)
}

// mapErr translates GORM errors into repository errors.
func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// uniqueViolation reports whether err is a unique-constraint violation
// (SQLSTATE 23505). It translates through the dialect locally instead of
// enabling gorm.Config.TranslateError, which would change the error text
// other endpoints echo to clients.
func uniqueViolation(db *gorm.DB, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	if translator, ok := db.Dialector.(gorm.ErrorTranslator); ok && errors.Is(translator.Translate(err), gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505")
}

// inTx runs fn on the transaction carried by ctx, or in a new transaction.
func (b base) inTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(tx)
	}
	return b.db.WithContext(ctx).Transaction(fn)
}
