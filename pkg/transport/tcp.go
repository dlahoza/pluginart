package transport

import (
	"context"
	"fmt"
	"net"
)

type tcpDialer struct {
	addr string
}

// NewTCP returns a Dialer that connects over TCP.
// If addr is empty a free port on 127.0.0.1 is allocated automatically.
func NewTCP(addr string) (Dialer, error) {
	if addr == "" {
		// Bind to :0 to let the OS assign a free port, then release it.
		// There is a brief TOCTOU window, but it is acceptable for plugin use.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("allocate tcp port: %w", err)
		}
		addr = l.Addr().String()
		_ = l.Close()
	}
	return &tcpDialer{addr: addr}, nil
}

func (d *tcpDialer) Addr() string {
	return d.addr
}

func (d *tcpDialer) Dial(ctx context.Context) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, "tcp", d.addr)
}
