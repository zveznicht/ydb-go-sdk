package retry

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xerrors"
)

type doOptions struct {
	retryOptions []Option
}

// doTxOption defines option for redefine default Retry behavior
type doOption interface {
	ApplyDoOption(opts *doOptions)
}

var (
	_ doOption = doRetryOptionsOption(nil)
	_ doOption = idOption("")
)

type doRetryOptionsOption []Option

func (retryOptions doRetryOptionsOption) ApplyDoOption(opts *doOptions) {
	opts.retryOptions = append(opts.retryOptions, retryOptions...)
}

// WithDoRetryOptions specified retry options
// Deprecated: use implicit options instead
func WithDoRetryOptions(opts ...Option) doRetryOptionsOption {
	return opts
}

// Do is a retryer of database/sql Conn with fallbacks on errors
func Do(ctx context.Context, db *sql.DB, f func(ctx context.Context, cc *sql.Conn) error, opts ...doOption) error {
	var (
		options  = doOptions{}
		attempts = 0
	)
	for _, opt := range opts {
		if opt != nil {
			opt.ApplyDoOption(&options)
		}
	}
	err := Retry(ctx, func(ctx context.Context) (err error) {
		attempts++
		cc, err := db.Conn(ctx)
		if err != nil {
			return unwrapErrBadConn(xerrors.WithStackTrace(err))
		}
		defer func() {
			_ = cc.Close()
		}()
		if err = f(ctx, cc); err != nil {
			return unwrapErrBadConn(xerrors.WithStackTrace(err))
		}
		return nil
	}, options.retryOptions...)
	if err != nil {
		return xerrors.WithStackTrace(
			fmt.Errorf("operation failed with %d attempts: %w", attempts, err),
		)
	}
	return nil
}

type doTxOptions struct {
	txOptions    *sql.TxOptions
	retryOptions []Option
}

// doTxOption defines option for redefine default Retry behavior
type doTxOption interface {
	ApplyDoTxOption(o *doTxOptions)
}

var _ doTxOption = doTxRetryOptionsOption(nil)

type doTxRetryOptionsOption []Option

func (doTxRetryOptions doTxRetryOptionsOption) ApplyDoTxOption(o *doTxOptions) {
	o.retryOptions = append(o.retryOptions, doTxRetryOptions...)
}

// WithDoTxRetryOptions specified retry options
// Deprecated: use implicit options instead
func WithDoTxRetryOptions(opts ...Option) doTxRetryOptionsOption {
	return opts
}

var _ doTxOption = txOptionsOption{}

type txOptionsOption struct {
	txOptions *sql.TxOptions
}

func (txOptions txOptionsOption) ApplyDoTxOption(o *doTxOptions) {
	o.txOptions = txOptions.txOptions
}

// WithTxOptions specified transaction options
func WithTxOptions(txOptions *sql.TxOptions) txOptionsOption {
	return txOptionsOption{
		txOptions: txOptions,
	}
}

// DoTx is a retryer of database/sql transactions with fallbacks on errors
func DoTx(ctx context.Context, db *sql.DB, f func(context.Context, *sql.Tx) error, opts ...doTxOption) error {
	var (
		options = doTxOptions{
			txOptions: &sql.TxOptions{
				Isolation: sql.LevelDefault,
				ReadOnly:  false,
			},
		}
		attempts = 0
	)
	for _, opt := range opts {
		if opt != nil {
			opt.ApplyDoTxOption(&options)
		}
	}
	err := Retry(ctx, func(ctx context.Context) (err error) {
		attempts++
		tx, err := db.BeginTx(ctx, options.txOptions)
		if err != nil {
			return unwrapErrBadConn(xerrors.WithStackTrace(err))
		}
		defer func() {
			if err != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					err = xerrors.NewWithIssues("",
						xerrors.WithStackTrace(err),
						xerrors.WithStackTrace(errRollback),
					)
				} else {
					err = xerrors.WithStackTrace(err)
				}
			}
		}()
		if err = f(ctx, tx); err != nil {
			return unwrapErrBadConn(xerrors.WithStackTrace(err))
		}
		if err = tx.Commit(); err != nil {
			return unwrapErrBadConn(xerrors.WithStackTrace(err))
		}
		return nil
	}, options.retryOptions...)
	if err != nil {
		return xerrors.WithStackTrace(
			fmt.Errorf("tx operation failed with %d attempts: %w", attempts, err),
		)
	}
	return nil
}
