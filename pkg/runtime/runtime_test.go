package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dlahoza/pluginart/pkg/protocol"
	"github.com/dlahoza/pluginart/pkg/transport"
)

type testHandler func(context.Context, []byte) ([]byte, error)

func (f testHandler) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	return f(ctx, payload)
}

func TestDurationUnmarshalText(t *testing.T) {
	var d duration
	if err := d.UnmarshalText([]byte("150ms")); err != nil {
		t.Fatal(err)
	}
	if d.Duration != 150*time.Millisecond {
		t.Fatalf("duration = %v", d.Duration)
	}
	if d.val(time.Second) != 150*time.Millisecond {
		t.Fatalf("val = %v", d.val(time.Second))
	}

	var zero duration
	if zero.val(time.Second) != time.Second {
		t.Fatalf("zero val = %v", zero.val(time.Second))
	}

	if err := d.UnmarshalText([]byte("bad")); err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewManagerFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pluginart.toml")
	content := `
version = 1

[defaults]
startup_timeout = "1s"
shutdown_timeout = "2s"
health_interval = "3s"
max_restarts = 9
restart_backoff_max = "4s"

[[plugins]]
name = "one"
type = "remote"
address = "127.0.0.1:1"
contract_hash = "hash"

[[plugins]]
name = "two"
type = "binary"
path = "/bin/true"
startup_timeout = "5s"
max_restarts = 2
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := NewManagerFromConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.plugins) != 2 {
		t.Fatalf("plugins = %d", len(m.plugins))
	}
	if got := m.plugins["one"].startupTimeout; got != time.Second {
		t.Fatalf("default startup = %v", got)
	}
	if got := m.plugins["one"].maxRestarts; got != 9 {
		t.Fatalf("default max restarts = %d", got)
	}
	if got := m.plugins["two"].startupTimeout; got != 5*time.Second {
		t.Fatalf("plugin startup = %v", got)
	}
	if got := m.plugins["two"].maxRestarts; got != 2 {
		t.Fatalf("plugin max restarts = %d", got)
	}
}

func TestNewManagerFromConfigErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if _, err := NewManagerFromConfig(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad version", func(t *testing.T) {
		path := writeConfig(t, "version = 2\n")
		if _, err := NewManagerFromConfig(path); err == nil || !strings.Contains(err.Error(), "unsupported config version") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		path := writeConfig(t, `
version = 1
[[plugins]]
name = "dup"
[[plugins]]
name = "dup"
`)
		if _, err := NewManagerFromConfig(path); err == nil || !strings.Contains(err.Error(), "duplicate plugin name") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestNewPluginDefaults(t *testing.T) {
	p := newPlugin(&PluginConfig{Name: "p"}, DefaultsConfig{})
	if p.startupTimeout != defaultStartupTimeout {
		t.Fatalf("startup = %v", p.startupTimeout)
	}
	if p.shutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("shutdown = %v", p.shutdownTimeout)
	}
	if p.healthInterval != defaultHealthInterval {
		t.Fatalf("health = %v", p.healthInterval)
	}
	if p.maxRestarts != defaultMaxRestarts {
		t.Fatalf("maxRestarts = %d", p.maxRestarts)
	}
	if p.restartBackoffMax != defaultRestartBackoffMax {
		t.Fatalf("restartBackoffMax = %v", p.restartBackoffMax)
	}
	if p.restartBackoff != initialRestartBackoff {
		t.Fatalf("restartBackoff = %v", p.restartBackoff)
	}
}

func TestPluginNewDialer(t *testing.T) {
	tests := []struct {
		name string
		cfg  PluginConfig
		want string
	}{
		{name: "explicit tcp", cfg: PluginConfig{Name: "p", Transport: "tcp"}, want: "tcp"},
		{name: "remote", cfg: PluginConfig{Name: "p", Type: "remote", Address: "127.0.0.1:1"}, want: "127.0.0.1:1"},
		{name: "docker default tcp", cfg: PluginConfig{Name: "p", Type: "docker", Address: "127.0.0.1:2"}, want: "127.0.0.1:2"},
		{name: "default unix", cfg: PluginConfig{Name: "p"}, want: ".sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPlugin(&tt.cfg, DefaultsConfig{})
			d, err := p.newDialer()
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == "tcp" {
				if _, ok := d.(interface {
					Dial(context.Context) (net.Conn, error)
				}); !ok {
					t.Fatalf("dialer does not implement Dial")
				}
				if !strings.Contains(d.Addr(), "127.0.0.1:") {
					t.Fatalf("addr = %q", d.Addr())
				}
				return
			}
			if !strings.Contains(d.Addr(), tt.want) {
				t.Fatalf("addr = %q, want contains %q", d.Addr(), tt.want)
			}
		})
	}
}

func TestPluginStartUnsupported(t *testing.T) {
	p := newPlugin(&PluginConfig{Name: "p", Type: "bad"}, DefaultsConfig{})
	if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported plugin type") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginStartBinaryStartError(t *testing.T) {
	p := newPlugin(&PluginConfig{Name: "p", Type: "binary", Path: filepath.Join(t.TempDir(), "missing")}, DefaultsConfig{})
	if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "start") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginStartBinary(t *testing.T) {
	p := newPlugin(&PluginConfig{
		Name:         "p",
		Type:         "binary",
		Path:         os.Args[0],
		Args:         []string{"-test.run=TestRuntimeHelperProcess", "--"},
		ContractHash: "hash",
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "serve",
		},
	}, DefaultsConfig{HealthInterval: duration{Duration: time.Hour}})

	if err := p.start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(context.Background())

	got, err := p.call(context.Background(), []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "echo:request" {
		t.Fatalf("response = %q", got)
	}
	if p.cmd == nil || p.cancelHealth == nil {
		t.Fatalf("cmd/cancelHealth not set: %#v", p)
	}
}

func TestPluginStartBinaryTCP(t *testing.T) {
	p := newPlugin(&PluginConfig{
		Name:         "p",
		Type:         "binary",
		Transport:    "tcp",
		Path:         os.Args[0],
		Args:         []string{"-test.run=TestRuntimeHelperProcess", "--"},
		ContractHash: "hash",
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "serve",
		},
	}, DefaultsConfig{HealthInterval: duration{Duration: time.Hour}})

	if err := p.start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(context.Background())
	if p.client == nil {
		t.Fatal("client not set")
	}
}

func TestPluginStartBinaryExitsBeforeReady(t *testing.T) {
	p := newPlugin(&PluginConfig{
		Name: "p",
		Type: "binary",
		Path: os.Args[0],
		Args: []string{"-test.run=TestRuntimeHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "exit",
		},
	}, DefaultsConfig{})
	if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "exited before READY") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginStartBinaryStartupTimeout(t *testing.T) {
	p := newPlugin(&PluginConfig{
		Name: "p",
		Type: "binary",
		Path: os.Args[0],
		Args: []string{"-test.run=TestRuntimeHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "sleep",
		},
		StartupTimeout: duration{Duration: 25 * time.Millisecond},
	}, DefaultsConfig{})
	if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "startup timeout") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginStartRemote(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		protocol.NewServer(&singleListener{Conn: c}, testHandler(func(_ context.Context, payload []byte) ([]byte, error) {
			return payload, nil
		}), "hash").Serve()
	}()

	p := newPlugin(&PluginConfig{Name: "p", Type: "remote", Address: ln.Addr().String(), ContractHash: "hash"}, DefaultsConfig{
		HealthInterval: duration{Duration: time.Hour},
	})
	if err := p.start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(context.Background())
	if p.client == nil || p.cancelHealth == nil {
		t.Fatalf("client/cancelHealth not set: %#v", p)
	}
}

func TestPluginStartRemoteDialError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	p := newPlugin(&PluginConfig{Name: "p", Type: "remote", Address: addr, ContractHash: "hash"}, DefaultsConfig{})
	if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "dial") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginStartDocker(t *testing.T) {
	installFakeDocker(t, "success")
	p := newPlugin(&PluginConfig{
		Name:         "p",
		Type:         "docker",
		Image:        "image:local",
		ContractHash: "hash",
		Env:          map[string]string{"EXTRA": "value"},
		Resources:    ResourcesConfig{Memory: "64m", CPUs: "0.5"},
	}, DefaultsConfig{HealthInterval: duration{Duration: time.Hour}})

	if err := p.start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(context.Background())
	if p.containerID != "container-id" || p.client == nil {
		t.Fatalf("container/client not set: id=%q client=%v", p.containerID, p.client)
	}

	got, err := p.call(context.Background(), []byte("docker"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "echo:docker" {
		t.Fatalf("response = %q", got)
	}
}

func TestPluginStartDockerErrors(t *testing.T) {
	t.Run("run error", func(t *testing.T) {
		installFakeDocker(t, "run-error")
		p := newPlugin(&PluginConfig{Name: "p", Type: "docker", Image: "image:local"}, DefaultsConfig{})
		if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "docker run") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("logs before ready", func(t *testing.T) {
		installFakeDocker(t, "logs-exit")
		p := newPlugin(&PluginConfig{Name: "p", Type: "docker", Image: "image:local"}, DefaultsConfig{})
		if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "docker logs exited before READY") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("startup timeout", func(t *testing.T) {
		installFakeDocker(t, "logs-sleep")
		p := newPlugin(&PluginConfig{
			Name:           "p",
			Type:           "docker",
			Image:          "image:local",
			StartupTimeout: duration{Duration: 25 * time.Millisecond},
		}, DefaultsConfig{})
		if err := p.start(context.Background()); err == nil || !strings.Contains(err.Error(), "startup timeout") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPluginCall(t *testing.T) {
	client, closeClient := newProtocolClient(t, "hash", func(_ context.Context, payload []byte) ([]byte, error) {
		if string(payload) != "request" {
			t.Errorf("payload = %q", payload)
		}
		return []byte("response"), nil
	})
	defer closeClient()

	p := newPlugin(&PluginConfig{Name: "p"}, DefaultsConfig{})
	p.client = client

	got, err := p.call(context.Background(), []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "response" {
		t.Fatalf("response = %q", got)
	}
}

func TestPluginCallNotConnected(t *testing.T) {
	p := newPlugin(&PluginConfig{Name: "p"}, DefaultsConfig{})
	if _, err := p.call(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginConnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		protocol.NewServer(&singleListener{Conn: c}, testHandler(func(_ context.Context, payload []byte) ([]byte, error) {
			return payload, nil
		}), "hash").Serve()
	}()

	d, err := transport.NewTCP(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	p := newPlugin(&PluginConfig{Name: "p", ContractHash: "hash"}, DefaultsConfig{})
	if err := p.connect(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(context.Background())

	if p.client == nil {
		t.Fatal("client not set")
	}
}

func TestPluginConnectErrors(t *testing.T) {
	t.Run("dial", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()

		d, err := transport.NewTCP(addr)
		if err != nil {
			t.Fatal(err)
		}
		p := newPlugin(&PluginConfig{Name: "p", ContractHash: "hash"}, DefaultsConfig{})
		if err := p.connect(context.Background(), d); err == nil || !strings.Contains(err.Error(), "dial") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("handshake", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		go func() {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			protocol.NewServer(&singleListener{Conn: c}, testHandler(func(context.Context, []byte) ([]byte, error) {
				return nil, nil
			}), "other").Serve()
		}()

		d, err := transport.NewTCP(ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		p := newPlugin(&PluginConfig{Name: "p", ContractHash: "hash"}, DefaultsConfig{})
		if err := p.connect(context.Background(), d); err == nil || !strings.Contains(err.Error(), "handshake") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestManagerCallUnknownPlugin(t *testing.T) {
	m := &PluginManager{plugins: map[string]*plugin{}}
	if _, err := m.Call(context.Background(), "missing", nil); err == nil || !strings.Contains(err.Error(), "unknown plugin") {
		t.Fatalf("err = %v", err)
	}
}

func TestManagerStartAndShutdown(t *testing.T) {
	m := &PluginManager{plugins: map[string]*plugin{
		"bad": newPlugin(&PluginConfig{Name: "bad", Type: "unsupported"}, DefaultsConfig{}),
	}}
	if err := m.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported plugin type") {
		t.Fatalf("err = %v", err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestManagerStartSuccessAndCall(t *testing.T) {
	m := &PluginManager{plugins: map[string]*plugin{
		"p": newPlugin(&PluginConfig{
			Name:         "p",
			Type:         "binary",
			Path:         os.Args[0],
			Args:         []string{"-test.run=TestRuntimeHelperProcess", "--"},
			ContractHash: "hash",
			Env: map[string]string{
				"GO_WANT_HELPER_PROCESS": "serve",
			},
		}, DefaultsConfig{HealthInterval: duration{Duration: time.Hour}}),
	}}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())

	got, err := m.Call(context.Background(), "p", []byte("manager"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "echo:manager" {
		t.Fatalf("response = %q", got)
	}
}

func TestPluginShutdownClearsState(t *testing.T) {
	client, closeClient := newProtocolClient(t, "hash", func(context.Context, []byte) ([]byte, error) {
		return nil, nil
	})
	defer closeClient()

	cancelled := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	p := newPlugin(&PluginConfig{Name: "p"}, DefaultsConfig{})
	p.client = client
	p.cancelHealth = func() {
		cancel()
		close(cancelled)
	}

	if err := p.shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("health cancel not called")
	}
	if ctx.Err() == nil {
		t.Fatal("context not cancelled")
	}
	if p.client != nil || p.cancelHealth != nil || p.cmd != nil || p.containerID != "" {
		t.Fatalf("state not cleared: %#v", p)
	}
}

func TestPluginShutdownProcessKillAfterTimeout(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeHelperProcess", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=sleep")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	p := newPlugin(&PluginConfig{Name: "p", ShutdownTimeout: duration{Duration: 25 * time.Millisecond}}, DefaultsConfig{})
	p.cmd = cmd

	if err := p.shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPluginRunHealthCheckSuccess(t *testing.T) {
	client, closeClient := newProtocolClient(t, "hash", func(_ context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	})
	defer closeClient()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newPlugin(&PluginConfig{Name: "p", Type: "remote", HealthInterval: duration{Duration: 10 * time.Millisecond}}, DefaultsConfig{})
	p.client = client
	p.restartBackoff = 10 * time.Second

	done := make(chan struct{})
	go func() {
		p.runHealthCheck(ctx)
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		p.mu.Lock()
		backoff := p.restartBackoff
		p.mu.Unlock()
		if backoff == initialRestartBackoff {
			cancel()
			<-done
			return
		}
		select {
		case <-deadline:
			t.Fatal("health check did not reset backoff")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestPluginRunHealthCheckNoClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := newPlugin(&PluginConfig{Name: "p", HealthInterval: duration{Duration: 10 * time.Millisecond}}, DefaultsConfig{})

	done := make(chan struct{})
	go func() {
		p.runHealthCheck(ctx)
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("health check did not stop")
	}
}

func TestPluginRunHealthCheckFailedRemoteDoesNotRestart(t *testing.T) {
	client, closeClient := newProtocolClient(t, "hash", func(context.Context, []byte) ([]byte, error) {
		return nil, nil
	})
	closeClient()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newPlugin(&PluginConfig{Name: "p", Type: "remote", HealthInterval: duration{Duration: 10 * time.Millisecond}}, DefaultsConfig{})
	p.client = client

	done := make(chan struct{})
	go func() {
		p.runHealthCheck(ctx)
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("health check did not stop")
	}
	if p.restartCount != 0 {
		t.Fatalf("restartCount = %d", p.restartCount)
	}
}

func TestPluginRunHealthCheckFailedBinaryBranches(t *testing.T) {
	t.Run("max restarts exceeded", func(t *testing.T) {
		client, closeClient := newProtocolClient(t, "hash", func(context.Context, []byte) ([]byte, error) {
			return nil, nil
		})
		closeClient()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		p := newPlugin(&PluginConfig{Name: "p", Type: "binary", HealthInterval: duration{Duration: 10 * time.Millisecond}, MaxRestarts: 1}, DefaultsConfig{})
		p.client = client
		p.restartCount = 1

		done := make(chan struct{})
		go func() {
			p.runHealthCheck(ctx)
			close(done)
		}()

		time.Sleep(25 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("health check did not stop")
		}
		if p.restartCount != 1 {
			t.Fatalf("restartCount = %d", p.restartCount)
		}
	})

	t.Run("restart attempt fails", func(t *testing.T) {
		client, closeClient := newProtocolClient(t, "hash", func(context.Context, []byte) ([]byte, error) {
			return nil, nil
		})
		closeClient()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		p := newPlugin(&PluginConfig{
			Name:           "p",
			Type:           "binary",
			Path:           filepath.Join(t.TempDir(), "missing"),
			HealthInterval: duration{Duration: 10 * time.Millisecond},
			MaxRestarts:    2,
		}, DefaultsConfig{})
		p.client = client
		p.restartBackoff = time.Millisecond
		p.restartBackoffMax = time.Millisecond

		done := make(chan struct{})
		go func() {
			p.runHealthCheck(ctx)
			close(done)
		}()

		deadline := time.After(time.Second)
		for {
			p.mu.Lock()
			count := p.restartCount
			p.mu.Unlock()
			if count > 0 {
				cancel()
				<-done
				return
			}
			select {
			case <-deadline:
				t.Fatal("restart was not attempted")
			case <-time.After(5 * time.Millisecond):
			}
		}
	})
}

func TestRuntimeHelperProcess(_ *testing.T) {
	switch os.Getenv("GO_WANT_HELPER_PROCESS") {
	case "":
		return
	case "exit":
		os.Exit(0)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "serve":
	default:
		fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", os.Getenv("GO_WANT_HELPER_PROCESS"))
		os.Exit(2)
	}

	var (
		ln  net.Listener
		err error
	)
	switch {
	case os.Getenv("PLUGIN_SOCKET") != "":
		ln, err = net.Listen("unix", os.Getenv("PLUGIN_SOCKET"))
	case os.Getenv("PLUGIN_ADDR") != "":
		ln, err = net.Listen("tcp", os.Getenv("PLUGIN_ADDR"))
	default:
		fmt.Fprintln(os.Stderr, "PLUGIN_SOCKET or PLUGIN_ADDR must be set")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(2)
	}
	fmt.Println("READY")
	err = protocol.NewServer(ln, testHandler(func(_ context.Context, payload []byte) ([]byte, error) {
		return []byte("echo:" + string(payload)), nil
	}), "hash").Serve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(2)
	}
	os.Exit(0)
}

func installFakeDocker(t *testing.T, mode string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	script := `#!/bin/sh
set -eu
mode="${FAKE_DOCKER_MODE}"
cmd="${1:-}"
shift || true
case "$cmd" in
  run)
    if [ "$mode" = "run-error" ]; then
      echo "run failed" >&2
      exit 1
    fi
    addr=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-e" ]; then
        shift
        case "$1" in
          PLUGIN_ADDR=*) addr="${1#PLUGIN_ADDR=}" ;;
        esac
      fi
      shift || true
    done
    if [ "$mode" = "success" ]; then
      GO_WANT_HELPER_PROCESS=serve PLUGIN_ADDR="$addr" "$HELPER_BINARY" -test.run=TestRuntimeHelperProcess -- >/tmp/pluginart-fake-docker-helper.log 2>&1 &
    fi
    echo container-id
    ;;
  logs)
    if [ "$mode" = "logs-exit" ]; then
      exit 0
    fi
    if [ "$mode" = "logs-sleep" ]; then
      sleep 10
      exit 0
    fi
    sleep 0.1
    echo READY
    sleep 10
    ;;
  stop|rm)
    exit 0
    ;;
  *)
    echo "unexpected docker command: $cmd" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_DOCKER_MODE", mode)
	t.Setenv("HELPER_BINARY", os.Args[0])
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pluginart.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newProtocolClient(t *testing.T, hash string, handler func(context.Context, []byte) ([]byte, error)) (*protocol.Client, func()) {
	t.Helper()
	a, b := net.Pipe()
	done := make(chan struct{})
	go func() {
		protocol.NewServer(&singleListener{Conn: b}, testHandler(handler), hash).Serve()
		close(done)
	}()
	client, err := protocol.Connect(a, "p", hash)
	if err != nil {
		t.Fatal(err)
	}
	return client, func() {
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("server did not exit")
		}
	}
}

type singleListener struct {
	net.Conn
	used bool
}

func (l *singleListener) Accept() (net.Conn, error) {
	if l.used {
		return nil, net.ErrClosed
	}
	l.used = true
	return l.Conn, nil
}

func (l *singleListener) Close() error {
	return l.Conn.Close()
}

func (l *singleListener) Addr() net.Addr {
	return l.LocalAddr()
}
