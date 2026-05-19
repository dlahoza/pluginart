package transport

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTCPWithAddress(t *testing.T) {
	d, err := NewTCP("127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	if d.Addr() != "127.0.0.1:12345" {
		t.Fatalf("addr = %q", d.Addr())
	}
}

func TestNewTCPAllocatesAddressAndDials(t *testing.T) {
	d, err := NewTCP("")
	if err != nil {
		t.Fatal(err)
	}
	if d.Addr() == "" {
		t.Fatal("expected allocated address")
	}

	ln, err := net.Listen("tcp", d.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := d.Dial(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	select {
	case peer := <-accepted:
		if peer != nil {
			_ = peer.Close()
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestTCPDialError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	d, err := NewTCP(addr)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if c, err := d.Dial(ctx); err == nil {
		_ = c.Close()
		t.Fatal("expected dial error")
	}
}

func TestNewUnixWithPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin.sock")
	d := NewUnix(path)
	if d.Addr() != path {
		t.Fatalf("addr = %q, want %q", d.Addr(), path)
	}
}

func TestNewUnixGeneratesPath(t *testing.T) {
	d := NewUnix("")
	if !strings.HasSuffix(d.Addr(), ".sock") {
		t.Fatalf("addr = %q, want .sock suffix", d.Addr())
	}
	if !strings.Contains(d.Addr(), "pluginart") {
		t.Fatalf("addr = %q, want pluginart directory", d.Addr())
	}
}

func TestUnixDial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	d := NewUnix(path)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := d.Dial(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	select {
	case peer := <-accepted:
		if peer != nil {
			_ = peer.Close()
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
