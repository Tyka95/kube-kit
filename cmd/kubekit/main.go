// Package main is the entry point for the kubekit TUI.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui"
)

// Version is set at build time via -ldflags. The string after the `=` is
// updated automatically by release-please from .release-please-manifest.json.
var Version = "0.2.0-dev" // x-release-please-version

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("kubekit %s\n", Version)
			return
		case "--help", "-h":
			fmt.Println("kubekit — terminal UI for Kubernetes + AWS workflows")
			fmt.Println("Usage: kubekit")
			return
		}
	}

	app := tui.NewApp()

	// defer Cleanup so any active DB tunnels are torn down on every exit
	// path: graceful quit, Ctrl+C inside the TUI, signal received, error
	// return, even panic. Without this, the local kubectl port-forward
	// and the socat pod in the cluster outlive the process.
	defer app.Cleanup()

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Catch SIGTERM / SIGHUP (terminal closed, kill <pid>) and ask the
	// program to quit gracefully. bubbletea already handles SIGINT
	// (Ctrl+C) by translating it to a tea.KeyMsg, which our App.Update
	// converts to tea.Quit — so p.Run() returns and the deferred Cleanup
	// fires. SIGTERM/SIGHUP need an explicit bridge here.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kubekit: %v\n", err)
		os.Exit(1)
	}

	// Goodbye banner. Runs after bubbletea has released the alt-screen
	// and after defer app.Cleanup() has closed any open tunnels, so the
	// message appears in the user's normal terminal scrollback.
	fmt.Println("kubekit — see you soon 👋")
}
