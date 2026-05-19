//go:build bench

package bench

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dlahoza/pluginart/pkg/protocol"
	pluginruntime "github.com/dlahoza/pluginart/pkg/runtime"
)

const (
	benchContractHash = "sha256:pluginart-bench"
	benchPluginName   = "bench"
)

var benchSizes = []int{10, 1000, 10000}

type echoHandler struct{}

func (echoHandler) Handle(_ context.Context, payload []byte) ([]byte, error) {
	return payload, nil
}

func BenchmarkHostGoManager(b *testing.B) {
	addr, closeServer := startInProcessServer(b)
	defer closeServer()

	for _, size := range benchSizes {
		b.Run(fmt.Sprintf("payload_%d_bytes", size), func(b *testing.B) {
			manager := newRemoteManager(b, addr)
			payload := bytes.Repeat([]byte("x"), size)

			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := manager.Call(context.Background(), benchPluginName, payload)
				if err != nil {
					b.Fatalf("manager call: %v", err)
				}
				if len(resp) != len(payload) {
					b.Fatalf("response size = %d, want %d", len(resp), len(payload))
				}
			}
		})
	}
}

func BenchmarkHostGoProtocolClient(b *testing.B) {
	addr, closeServer := startInProcessServer(b)
	defer closeServer()

	for _, size := range benchSizes {
		b.Run(fmt.Sprintf("payload_%d_bytes", size), func(b *testing.B) {
			client := newProtocolClient(b, addr)
			defer client.Close()
			payload := bytes.Repeat([]byte("x"), size)

			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.Call(context.Background(), payload)
				if err != nil {
					b.Fatalf("protocol call: %v", err)
				}
				if len(resp) != len(payload) {
					b.Fatalf("response size = %d, want %d", len(resp), len(payload))
				}
			}
		})
	}
}

func BenchmarkPluginServers(b *testing.B) {
	for _, size := range benchSizes {
		b.Run(fmt.Sprintf("go_payload_%d_bytes", size), func(b *testing.B) {
			plugin := goBenchPlugin()
			addr, stop := startPluginProcess(b, plugin)
			defer stop()
			client := newProtocolClient(b, addr)
			defer client.Close()
			payload := bytes.Repeat([]byte("x"), size)

			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.Call(context.Background(), payload)
				if err != nil {
					b.Fatalf("protocol call: %v", err)
				}
				if len(resp) != len(payload) {
					b.Fatalf("response size = %d, want %d", len(resp), len(payload))
				}
			}
		})
	}
}

func TestBenchPluginServerMemoryGrowth(t *testing.T) {
	plugin := goBenchPlugin()
	addr, stop := startPluginProcess(t, plugin)
	defer stop()
	client := newProtocolClient(t, addr)
	defer client.Close()
	payload := bytes.Repeat([]byte("x"), 10000)

	for i := 0; i < 10; i++ {
		callAndCheck(t, client, payload)
	}

	before, ok := processRSS(plugin.cmd.Process.Pid)
	if !ok {
		t.Skip("process RSS is unavailable on this platform")
	}
	for i := 0; i < 500; i++ {
		callAndCheck(t, client, payload)
	}
	after, ok := processRSS(plugin.cmd.Process.Pid)
	if !ok {
		t.Skip("process RSS is unavailable on this platform")
	}
	if growth := after - before; growth > 32*1024*1024 {
		t.Fatalf("plugin RSS grew by %d bytes after calls, want <= 32 MiB", growth)
	}
}

func TestBenchHostGoMemoryGrowth(t *testing.T) {
	addr, closeServer := startInProcessServer(t)
	defer closeServer()
	manager := newRemoteManager(t, addr)
	payload := bytes.Repeat([]byte("x"), 10000)

	for i := 0; i < 10; i++ {
		if resp, err := manager.Call(context.Background(), benchPluginName, payload); err != nil {
			t.Fatalf("warmup call: %v", err)
		} else if len(resp) != len(payload) {
			t.Fatalf("response size = %d, want %d", len(resp), len(payload))
		}
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	for i := 0; i < 500; i++ {
		if resp, err := manager.Call(context.Background(), benchPluginName, payload); err != nil {
			t.Fatalf("call %d: %v", i, err)
		} else if len(resp) != len(payload) {
			t.Fatalf("response size = %d, want %d", len(resp), len(payload))
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&after)
	if growth := retainedHeapGrowth(before, after); growth > 8*1024*1024 {
		t.Fatalf("retained heap grew by %d bytes after calls, want <= 8 MiB", growth)
	}
}

func callAndCheck(tb testing.TB, client *protocol.Client, payload []byte) {
	tb.Helper()
	resp, err := client.Call(context.Background(), payload)
	if err != nil {
		tb.Fatalf("protocol call: %v", err)
	}
	if len(resp) != len(payload) {
		tb.Fatalf("response size = %d, want %d", len(resp), len(payload))
	}
}

func startInProcessServer(tb testing.TB) (string, func()) {
	tb.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	server := protocol.NewServer(ln, echoHandler{}, benchContractHash)
	done := make(chan error, 1)
	go func() {
		done <- server.Serve()
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		select {
		case err := <-done:
			if err != nil {
				tb.Fatalf("server stopped with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			tb.Fatalf("server did not stop")
		}
	}
}

func newRemoteManager(tb testing.TB, addr string) *pluginruntime.PluginManager {
	tb.Helper()
	configPath := filepath.Join(tb.TempDir(), "pluginart.toml")
	config := fmt.Sprintf(`version = 1

[defaults]
startup_timeout = "5s"
shutdown_timeout = "2s"
health_interval = "30s"
max_restarts = 0

[[plugins]]
name = "%s"
type = "remote"
address = "%s"
contract_hash = "%s"
`, benchPluginName, addr, benchContractHash)
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		tb.Fatalf("write config: %v", err)
	}
	manager, err := pluginruntime.NewManagerFromConfig(configPath)
	if err != nil {
		tb.Fatalf("load manager: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		tb.Fatalf("start manager: %v", err)
	}
	tb.Cleanup(func() {
		_ = manager.Shutdown(context.Background())
	})
	return manager
}

func newProtocolClient(tb testing.TB, addr string) *protocol.Client {
	tb.Helper()
	raw, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		tb.Fatalf("dial %s: %v", addr, err)
	}
	client, err := protocol.Connect(raw, benchPluginName, benchContractHash)
	if err != nil {
		_ = raw.Close()
		tb.Fatalf("handshake: %v", err)
	}
	return client
}

type benchPluginProcess struct {
	name string
	cmd  *exec.Cmd
}

func goBenchPlugin() benchPluginProcess {
	root := repoRoot()
	path := filepath.Join(root, "bench/bin/pluginart-bench-go-plugin")
	return benchPluginProcess{
		name: "go",
		cmd:  exec.Command(path),
	}.withDir(root).requireFile(path)
}

func (p benchPluginProcess) withDir(dir string) benchPluginProcess {
	p.cmd.Dir = dir
	return p
}

func (p benchPluginProcess) requireFile(path string) benchPluginProcess {
	if _, err := os.Stat(path); err != nil {
		p.cmd = nil
	}
	return p
}

func startPluginProcess(tb testing.TB, plugin benchPluginProcess) (string, func()) {
	tb.Helper()
	if plugin.cmd == nil {
		tb.Skipf("%s benchmark plugin is not built", plugin.name)
	}
	addr := freeTCPAddr(tb)
	env := plugin.cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	plugin.cmd.Env = append(env, "PLUGIN_ADDR="+addr)
	stdout, err := plugin.cmd.StdoutPipe()
	if err != nil {
		tb.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	plugin.cmd.Stderr = &stderr
	if err := plugin.cmd.Start(); err != nil {
		tb.Fatalf("start %s plugin: %v", plugin.name, err)
	}
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "READY" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			_ = plugin.cmd.Process.Kill()
			tb.Fatalf("%s plugin exited before READY: %s", plugin.name, stderr.String())
		}
	case <-time.After(15 * time.Second):
		_ = plugin.cmd.Process.Kill()
		tb.Fatalf("%s plugin startup timeout: %s", plugin.name, stderr.String())
	}
	return addr, func() {
		if plugin.cmd.Process != nil {
			_ = plugin.cmd.Process.Kill()
		}
		done := make(chan error, 1)
		go func() {
			done <- plugin.cmd.Wait()
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			tb.Fatalf("%s plugin did not stop", plugin.name)
		}
	}
}

func freeTCPAddr(tb testing.TB) string {
	tb.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("allocate tcp port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		tb.Fatalf("close temp listener: %v", err)
	}
	return addr
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Clean(filepath.Join(wd, "../.."))
}

func retainedHeapGrowth(before, after runtime.MemStats) uint64 {
	if after.HeapAlloc <= before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}

func processRSS(pid int) (int64, bool) {
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, err := strconv.ParseInt(fields[1], 10, 64)
					return kb * 1024, err == nil
				}
			}
		}
	}
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, false
	}
	kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kb * 1024, err == nil
}
