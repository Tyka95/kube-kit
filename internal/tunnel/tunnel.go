package tunnel

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Config holds the parameters for a socat-pod tunnel.
type Config struct {
	// Namespace is the kubectl namespace. Defaults to "default" if empty.
	Namespace string
	// Host is the remote DB host that the socat pod will connect to.
	Host string
	// RemotePort is the port on the remote host (e.g. 5432).
	RemotePort int
	// LocalPort is the local port to bind via kubectl port-forward (e.g. 15432).
	LocalPort int
	// Image is the socat container image. Defaults to "alpine/socat" if empty.
	Image string
}

// withDefaults returns a copy of c with zero-value fields filled in.
func (c Config) withDefaults() Config {
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.Image == "" {
		c.Image = "alpine/socat"
	}
	return c
}

// Tunnel represents an active socat-pod + kubectl port-forward session.
type Tunnel struct {
	podName string
	pfCmd   *exec.Cmd
	cfg     Config

	mu     sync.Mutex
	closed bool
}

// Open creates the socat pod in the cluster, waits for it to be Ready, then
// starts kubectl port-forward. The returned *Tunnel must be closed by the
// caller via Close(). Cancelling ctx kills the port-forward and deletes the
// pod.
//
// status receives human-readable progress messages ("creating pod…",
// "waiting for pod…", "tunnel active"). Messages are dropped (non-blocking)
// when the channel is full or nil.
func Open(ctx context.Context, cfg Config, status chan<- string) (*Tunnel, error) {
	cfg = cfg.withDefaults()

	podName := genPodName()

	send(status, "creating pod...")

	// kubectl run <pod> -n <ns> --image=<img> --restart=Never -- TCP-LISTEN:<rport>,fork TCP:<host>:<rport>
	createArgs := []string{
		"run", podName,
		"-n", cfg.Namespace,
		fmt.Sprintf("--image=%s", cfg.Image),
		"--restart=Never",
		"--",
		fmt.Sprintf("TCP-LISTEN:%d,fork", cfg.RemotePort),
		fmt.Sprintf("TCP:%s:%d", cfg.Host, cfg.RemotePort),
	}
	createCmd := exec.CommandContext(ctx, "kubectl", createArgs...)
	createCmd.Stdout = nil
	createCmd.Stderr = nil
	if out, err := createCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("kubectl run failed: %w\n%s", err, string(out))
	}

	send(status, "waiting for pod...")

	// kubectl wait --for=condition=Ready pod/<pod> -n <ns> --timeout=60s
	waitArgs := []string{
		"wait",
		"--for=condition=Ready",
		fmt.Sprintf("pod/%s", podName),
		"-n", cfg.Namespace,
		"--timeout=60s",
	}
	waitCmd := exec.CommandContext(ctx, "kubectl", waitArgs...)
	waitCmd.Stdout = nil
	waitCmd.Stderr = nil
	if out, err := waitCmd.CombinedOutput(); err != nil {
		// Best-effort cleanup before returning the error.
		_ = deletePod(podName, cfg.Namespace)
		return nil, fmt.Errorf("kubectl wait failed: %w\n%s", err, string(out))
	}

	// kubectl port-forward pod/<pod> -n <ns> <lport>:<rport>
	pfArgs := []string{
		"port-forward",
		fmt.Sprintf("pod/%s", podName),
		"-n", cfg.Namespace,
		fmt.Sprintf("%d:%d", cfg.LocalPort, cfg.RemotePort),
	}
	pfCmd := exec.CommandContext(ctx, "kubectl", pfArgs...)
	pfCmd.Stdout = nil
	pfCmd.Stderr = nil

	if err := pfCmd.Start(); err != nil {
		_ = deletePod(podName, cfg.Namespace)
		return nil, fmt.Errorf("kubectl port-forward failed to start: %w", err)
	}

	send(status, "tunnel active")

	return &Tunnel{
		podName: podName,
		pfCmd:   pfCmd,
		cfg:     cfg,
	}, nil
}

// Close stops the port-forward process and deletes the socat pod.
// It is idempotent; subsequent calls are no-ops.
func (t *Tunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	// Kill the port-forward process.
	if t.pfCmd != nil && t.pfCmd.Process != nil {
		_ = t.pfCmd.Process.Kill()
		_ = t.pfCmd.Wait()
	}

	// Delete the socat pod with a bounded timeout.
	return deletePod(t.podName, t.cfg.Namespace)
}

// Wait blocks until the underlying port-forward process exits, either because
// the cluster connection was lost or because Close was called.
func (t *Tunnel) Wait() error {
	if t.pfCmd == nil {
		return nil
	}
	return t.pfCmd.Wait()
}

// deletePod runs kubectl delete pod with a 10s timeout. It is safe to call
// even after the pod is already gone (--ignore-not-found).
func deletePod(podName, namespace string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"kubectl", "delete", "pod", podName,
		"-n", namespace,
		"--ignore-not-found",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl delete pod %s: %w\n%s", podName, err, string(out))
	}
	return nil
}

// genPodName returns a unique pod name incorporating the current PID and a
// 6-character random hex suffix.
func genPodName() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return fmt.Sprintf("kubekit-tunnel-%d-%s", os.Getpid(), string(b))
}

// send emits msg on ch in a non-blocking way. It is a no-op if ch is nil.
func send(ch chan<- string, msg string) {
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}
