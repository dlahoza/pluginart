package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	repeat "example-host/plugins/repeat"

	pluginruntime "github.com/dlahoza/pluginart/pkg/runtime"
)

type repeatBenchmarkPlugin struct {
	name  string
	image string
}

var repeatBenchmarkPlugins = []repeatBenchmarkPlugin{
	{name: "repeat-go", image: "pluginart-repeat-go:local"},
	{name: "repeat-py", image: "pluginart-repeat-py:local"},
	{name: "repeat-ts", image: "pluginart-repeat-ts:local"},
}

const repeatBenchmarkConfigTmpl = `version = 1

[defaults]
startup_timeout     = "10s"
shutdown_timeout    = "5s"
health_interval     = "30s"
max_restarts        = 0
restart_backoff_max = "1s"
max_message_bytes   = 1048576

[[plugins]]
name = "%s"
type = "docker"
image = "%s"
contract_hash = "sha256:f83b4e6033244f2dbdcba6d61e07a7eed3e0a3cc49eb41ebdb57165231adaf8d"
`

func BenchmarkRepeatDocker(b *testing.B) {
	for _, plugin := range repeatBenchmarkPlugins {
		b.Run(plugin.name, func(b *testing.B) {
			for _, size := range []int{10, 1000, 10000} {
				b.Run(fmt.Sprintf("response_%d_bytes", size), func(b *testing.B) {
					manager := newRepeatBenchmarkManager(b, plugin)
					input := strings.Repeat("x", size)
					payload := buildRepeatCallPayload(input, 1)

					b.ReportAllocs()
					b.SetBytes(int64(size))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						raw, err := manager.Call(context.Background(), plugin.name, payload)
						if err != nil {
							b.Fatalf("repeat call: %v", err)
						}
						resp, _, err := repeat.DecodeRepeatResponse(raw)
						if err != nil {
							b.Fatalf("decode repeat response: %v", err)
						}
						if got := len(resp.Output()); got != size {
							b.Fatalf("response size = %d, want %d", got, size)
						}
					}
				})
			}
		})
	}
}

func BenchmarkRepeatDockerComparison(b *testing.B) {
	for _, size := range []int{10, 1000, 10000} {
		b.Run(fmt.Sprintf("response_%d_bytes", size), func(b *testing.B) {
			for _, plugin := range repeatBenchmarkPlugins {
				b.Run(plugin.name, func(b *testing.B) {
					manager := newRepeatBenchmarkManager(b, plugin)
					input := strings.Repeat("x", size)
					payload := buildRepeatCallPayload(input, 1)

					b.ReportAllocs()
					b.SetBytes(int64(size))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						raw, err := manager.Call(context.Background(), plugin.name, payload)
						if err != nil {
							b.Fatalf("repeat call: %v", err)
						}
						resp, _, err := repeat.DecodeRepeatResponse(raw)
						if err != nil {
							b.Fatalf("decode repeat response: %v", err)
						}
						if got := len(resp.Output()); got != size {
							b.Fatalf("response size = %d, want %d", got, size)
						}
					}
				})
			}
		})
	}
}

func TestRepeatDockerMemoryGrowth(t *testing.T) {
	for _, plugin := range repeatBenchmarkPlugins {
		t.Run(plugin.name, func(t *testing.T) {
			manager := newRepeatBenchmarkManager(t, plugin)
			input := strings.Repeat("x", 10000)
			payload := buildRepeatCallPayload(input, 1)

			for i := 0; i < 10; i++ {
				raw, err := manager.Call(context.Background(), plugin.name, payload)
				if err != nil {
					t.Fatalf("warmup repeat call: %v", err)
				}
				if _, _, err := repeat.DecodeRepeatResponse(raw); err != nil {
					t.Fatalf("decode warmup repeat response: %v", err)
				}
			}

			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)

			for i := 0; i < 500; i++ {
				raw, err := manager.Call(context.Background(), plugin.name, payload)
				if err != nil {
					t.Fatalf("repeat call %d: %v", i, err)
				}
				resp, _, err := repeat.DecodeRepeatResponse(raw)
				if err != nil {
					t.Fatalf("decode repeat response %d: %v", i, err)
				}
				if got := len(resp.Output()); got != 10000 {
					t.Fatalf("response size = %d, want 10000", got)
				}
			}

			runtime.GC()
			runtime.ReadMemStats(&after)
			if growth := retainedHeapGrowth(before, after); growth > 8*1024*1024 {
				t.Fatalf("retained heap grew by %d bytes after repeat calls, want <= 8 MiB", growth)
			}
		})
	}
}

func buildRepeatCallPayload(input string, count uint32) []byte {
	builder, payload := buildRepeatPayload(input, count)
	return repeat.BuildRepeatCallRequest(builder, payload)
}

func newRepeatBenchmarkManager(tb testing.TB, plugin repeatBenchmarkPlugin) *pluginruntime.PluginManager {
	tb.Helper()
	requireRepeatDockerImage(tb, plugin.image)

	dir := tb.TempDir()
	configPath := filepath.Join(dir, "pluginart.toml")
	config := fmt.Sprintf(repeatBenchmarkConfigTmpl, plugin.name, plugin.image)
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		tb.Fatalf("write benchmark config: %v", err)
	}

	manager, err := pluginruntime.NewManagerFromConfig(configPath)
	if err != nil {
		tb.Fatalf("load benchmark config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		tb.Fatalf("start repeat plugin: %v", err)
	}
	tb.Cleanup(func() {
		manager.Shutdown(context.Background())
	})

	return manager
}

func requireRepeatDockerImage(tb testing.TB, image string) {
	tb.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		tb.Skip("docker not available")
	}
	cmd := exec.Command("docker", "image", "inspect", image)
	if err := cmd.Run(); err != nil {
		tb.Skipf("docker image %s not available", image)
	}
}

func retainedHeapGrowth(before, after runtime.MemStats) uint64 {
	if after.HeapAlloc <= before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}
