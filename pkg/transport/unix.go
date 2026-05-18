package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

type unixDialer struct {
	path string
}

// NewUnix returns a Dialer that connects over a Unix socket.
// If path is empty a temporary socket path is generated under /tmp/pluginart/.
func NewUnix(path string) Dialer {
	if path == "" {
		path = generateUnixPath()
	}
	return &unixDialer{path: path}
}

func (d *unixDialer) Addr() string {
	return d.path
}

func (d *unixDialer) Dial(ctx context.Context) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, "unix", d.path)
}

func generateUnixPath() string {
	dir := "/tmp/pluginart"
	if err := os.MkdirAll(dir, 0o700); err != nil {
		// fall back to os.TempDir if /tmp/pluginart is not writable
		dir = filepath.Join(os.TempDir(), "pluginart")
		_ = os.MkdirAll(dir, 0o700)
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s/%s.sock", dir, hex.EncodeToString(b[:]))
}
