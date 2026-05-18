// Package transport provides Dialer implementations for Unix socket and TCP connections.
package transport

import (
	"context"
	"net"
)

// Dialer dials the plugin endpoint and returns a raw connection.
// Addr returns the address string injected into the plugin process via PLUGIN_SOCKET or PLUGIN_ADDR.
type Dialer interface {
	Dial(ctx context.Context) (net.Conn, error)
	Addr() string
}
