package runtime

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dlahoza/pluginart/pkg/protocol"
	"github.com/dlahoza/pluginart/pkg/transport"
)

const (
	defaultStartupTimeout    = 5 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
	defaultHealthInterval    = 2 * time.Second
	defaultMaxRestarts       = 5
	defaultRestartBackoffMax = 30 * time.Second
	defaultMaxMessageBytes   = 4 * 1024 * 1024
	initialRestartBackoff    = time.Second
	healthPingTimeout        = 2 * time.Second
)

type plugin struct {
	cfg      PluginConfig
	defaults DefaultsConfig

	startupTimeout    time.Duration
	shutdownTimeout   time.Duration
	healthInterval    time.Duration
	maxRestarts       int
	restartBackoffMax time.Duration
	restartBackoff    time.Duration
	restartCount      int

	client       *protocol.Client
	cmd          *exec.Cmd
	cmdDone      <-chan error
	cancelHealth context.CancelFunc
	containerID  string
	mu           sync.Mutex
}

func newPlugin(pc *PluginConfig, d DefaultsConfig) *plugin {
	p := &plugin{
		cfg:      *pc,
		defaults: d,
	}

	p.startupTimeout = pc.StartupTimeout.val(d.StartupTimeout.val(defaultStartupTimeout))
	p.shutdownTimeout = pc.ShutdownTimeout.val(d.ShutdownTimeout.val(defaultShutdownTimeout))
	p.healthInterval = pc.HealthInterval.val(d.HealthInterval.val(defaultHealthInterval))

	p.maxRestarts = pc.MaxRestarts
	if p.maxRestarts == 0 {
		p.maxRestarts = d.MaxRestarts
	}
	if p.maxRestarts == 0 {
		p.maxRestarts = defaultMaxRestarts
	}

	p.restartBackoffMax = pc.RestartBackoffMax.val(d.RestartBackoffMax.val(defaultRestartBackoffMax))
	p.restartBackoff = initialRestartBackoff

	return p
}

func (p *plugin) newDialer() (transport.Dialer, error) {
	switch {
	case p.cfg.Transport == "tcp":
		return transport.NewTCP("")
	case p.cfg.Type == "remote":
		return transport.NewTCP(p.cfg.Address)
	case p.cfg.Type == "docker" && p.cfg.Transport == "":
		return transport.NewTCP(p.cfg.Address)
	default:
		return transport.NewUnix(""), nil
	}
}

func (p *plugin) connect(ctx context.Context, dialer transport.Dialer) error {
	conn, err := dialer.Dial(ctx)
	if err != nil {
		return fmt.Errorf("dial %s: %w", dialer.Addr(), err)
	}
	client, err := protocol.Connect(conn, p.cfg.Name, p.cfg.ContractHash)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("handshake with %q: %w", p.cfg.Name, err)
	}
	p.mu.Lock()
	p.client = client
	p.mu.Unlock()
	return nil
}

func (p *plugin) start(ctx context.Context) error {
	switch p.cfg.Type {
	case "binary", "":
		return p.startBinary(ctx)
	case "remote":
		return p.startRemote(ctx)
	case "docker":
		return p.startDocker(ctx)
	default:
		return fmt.Errorf("unsupported plugin type %q", p.cfg.Type)
	}
}

func (p *plugin) startBinary(ctx context.Context) error {
	dialer, err := p.newDialer()
	if err != nil {
		return fmt.Errorf("create dialer for %q: %w", p.cfg.Name, err)
	}

	var socketEnv string
	if p.cfg.Transport == "tcp" {
		socketEnv = "PLUGIN_ADDR=" + dialer.Addr()
	} else {
		socketEnv = "PLUGIN_SOCKET=" + dialer.Addr()
	}

	envVars := os.Environ()
	envVars = append(envVars, socketEnv)
	for k, v := range p.cfg.Env {
		envVars = append(envVars, k+"="+v)
	}

	cmd := exec.CommandContext(ctx, p.cfg.Path, p.cfg.Args...)
	cmd.Env = envVars

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe for %q: %w", p.cfg.Name, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", p.cfg.Name, err)
	}

	ready := make(chan struct{}, 1)
	procDone := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "READY" {
				ready <- struct{}{}
				return
			}
		}
	}()

	go func() {
		procDone <- cmd.Wait()
	}()

	startCtx, cancel := context.WithTimeout(ctx, p.startupTimeout)
	defer cancel()

	select {
	case <-ready:
	case err := <-procDone:
		if err != nil {
			return fmt.Errorf("plugin %q exited before READY: %w", p.cfg.Name, err)
		}
		return fmt.Errorf("plugin %q exited before READY", p.cfg.Name)
	case <-startCtx.Done():
		_ = cmd.Process.Kill()
		return fmt.Errorf("plugin %q startup timeout", p.cfg.Name)
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, p.startupTimeout)
	defer dialCancel()

	if err := p.connect(dialCtx, dialer); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	p.mu.Lock()
	p.cmd = cmd
	p.cmdDone = procDone
	p.mu.Unlock()

	hctx, hcancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancelHealth = hcancel
	p.mu.Unlock()
	go p.runHealthCheck(hctx)

	return nil
}

func (p *plugin) startRemote(ctx context.Context) error {
	dialTimeout := p.cfg.DialTimeout.val(defaultStartupTimeout)
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	dialer, err := transport.NewTCP(p.cfg.Address)
	if err != nil {
		return fmt.Errorf("create tcp dialer for %q: %w", p.cfg.Name, err)
	}

	if err := p.connect(dialCtx, dialer); err != nil {
		return err
	}

	hctx, hcancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancelHealth = hcancel
	p.mu.Unlock()
	go p.runHealthCheck(hctx)

	return nil
}

func (p *plugin) startDocker(ctx context.Context) error {
	dialer, err := transport.NewTCP("")
	if err != nil {
		return fmt.Errorf("allocate tcp port for %q: %w", p.cfg.Name, err)
	}
	host, port, err := net.SplitHostPort(dialer.Addr())
	if err != nil {
		return fmt.Errorf("allocate tcp port for %q: %w", p.cfg.Name, err)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	containerAddr := "0.0.0.0:" + port

	args := []string{"run", "-d", "-p", host + ":" + port + ":" + port, "-e", "PLUGIN_ADDR=" + containerAddr}
	for k, v := range p.cfg.Env {
		args = append(args, "-e", k+"="+v)
	}
	if p.cfg.Resources.Memory != "" {
		args = append(args, "--memory", p.cfg.Resources.Memory)
	}
	if p.cfg.Resources.CPUs != "" {
		args = append(args, "--cpus", p.cfg.Resources.CPUs)
	}
	args = append(args, p.cfg.Image)
	args = append(args, p.cfg.Args...)

	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return fmt.Errorf("docker run %q: %w", p.cfg.Name, err)
	}
	containerID := strings.TrimSpace(string(out))

	logsCmd := exec.Command("docker", "logs", "-f", containerID)
	logsStdout, err := logsCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("docker logs pipe for %q: %w", p.cfg.Name, err)
	}
	if err := logsCmd.Start(); err != nil {
		return fmt.Errorf("docker logs start for %q: %w", p.cfg.Name, err)
	}

	ready := make(chan struct{}, 1)
	procDone := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(logsStdout)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "READY" {
				ready <- struct{}{}
				return
			}
		}
	}()

	go func() {
		procDone <- logsCmd.Wait()
	}()

	startCtx, cancel := context.WithTimeout(ctx, p.startupTimeout)
	defer cancel()

	select {
	case <-ready:
	case err := <-procDone:
		if err != nil {
			return fmt.Errorf("plugin %q docker logs exited before READY: %w", p.cfg.Name, err)
		}
		return fmt.Errorf("plugin %q docker logs exited before READY", p.cfg.Name)
	case <-startCtx.Done():
		_ = logsCmd.Process.Kill()
		return fmt.Errorf("plugin %q startup timeout", p.cfg.Name)
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, p.startupTimeout)
	defer dialCancel()

	if err := p.connect(dialCtx, dialer); err != nil {
		return err
	}

	p.mu.Lock()
	p.containerID = containerID
	p.mu.Unlock()

	hctx, hcancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancelHealth = hcancel
	p.mu.Unlock()
	go p.runHealthCheck(hctx)

	return nil
}

func (p *plugin) runHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(p.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			client := p.client
			p.mu.Unlock()

			if client == nil {
				continue
			}

			pingCtx, cancel := context.WithTimeout(ctx, healthPingTimeout)
			err := client.Ping(pingCtx)
			cancel()

			if err == nil {
				p.mu.Lock()
				p.restartBackoff = initialRestartBackoff
				p.mu.Unlock()
				continue
			}

			fmt.Fprintf(os.Stderr, "pluginart: health check failed for %q: %v\n", p.cfg.Name, err)

			p.mu.Lock()
			restarts := p.restartCount
			maxRestarts := p.maxRestarts
			p.mu.Unlock()

			if p.cfg.Type != "binary" {
				continue
			}

			if restarts >= maxRestarts {
				fmt.Fprintf(os.Stderr, "pluginart: plugin %q exceeded max restarts (%d)\n", p.cfg.Name, maxRestarts)
				continue
			}

			p.mu.Lock()
			backoff := p.restartBackoff
			p.restartBackoff *= 2
			if p.restartBackoff > p.restartBackoffMax {
				p.restartBackoff = p.restartBackoffMax
			}
			p.restartCount++
			p.mu.Unlock()

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			if err := p.start(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "pluginart: restart of %q failed: %v\n", p.cfg.Name, err)
			}
		}
	}
}

func (p *plugin) call(ctx context.Context, payload []byte) ([]byte, error) {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("plugin %q not connected", p.cfg.Name)
	}
	return client.Call(ctx, payload)
}

func (p *plugin) shutdown(ctx context.Context) error {
	p.mu.Lock()
	cancel := p.cancelHealth
	client := p.client
	cmd := p.cmd
	cmdDone := p.cmdDone
	containerID := p.containerID
	p.client = nil
	p.cmd = nil
	p.cmdDone = nil
	p.cancelHealth = nil
	p.containerID = ""
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if client != nil {
		_ = client.Close()
	}

	if containerID != "" {
		shutCtx, shutCancel := context.WithTimeout(ctx, p.shutdownTimeout)
		defer shutCancel()
		_ = exec.CommandContext(shutCtx, "docker", "stop", "-t", "1", containerID).Run()
		_ = exec.Command("docker", "rm", containerID).Run()
		return nil
	}

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
		return nil
	}

	done := cmdDone
	if done == nil {
		fallbackDone := make(chan error, 1)
		go func() { fallbackDone <- cmd.Wait() }()
		done = fallbackDone
	}

	shutCtx, shutCancel := context.WithTimeout(ctx, p.shutdownTimeout)
	defer shutCancel()

	select {
	case <-done:
	case <-shutCtx.Done():
		_ = cmd.Process.Kill()
		<-done
	}

	return nil
}
