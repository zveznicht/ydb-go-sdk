package balancer

import (
	"context"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/conn"
)

// Balancer is an interface that implements particular load-balancing
// algorithm.
//
// Balancer methods called synchronized. That is, implementations must not
// provide additional goroutine safety.
type Balancer interface {
	// Next returns next connection for request.
	Next(ctx context.Context, opts ...NextOption) conn.Conn

	// Create same balancer instance with new connections
	Create(conns []conn.Conn) Balancer
}

func IsOkConnection(c conn.Conn, bannedIsOk bool) bool {
	switch c.GetState() {
	case conn.Online, conn.Created, conn.Offline:
		return true
	case conn.Banned:
		return bannedIsOk
	default:
		return false
	}
}

type (
	NextOption         func(o *NextOptions)
	OnBadStateCallback func(ctx context.Context)

	NextOptions struct {
		OnBadState   OnBadStateCallback
		AcceptBanned bool
	}
)

func MakeNextOptions(opts ...NextOption) NextOptions {
	var o NextOptions
	for _, f := range opts {
		f(&o)
	}
	return o
}

func (o *NextOptions) Discovery(ctx context.Context) {
	if o.OnBadState != nil {
		o.OnBadState(ctx)
	}
}

func WithAcceptBanned(val bool) NextOption {
	return func(o *NextOptions) {
		o.AcceptBanned = val
	}
}

func WithOnBadState(callback OnBadStateCallback) NextOption {
	return func(o *NextOptions) {
		o.OnBadState = callback
	}
}
