// Package main is the entry point for the kubekit TUI.
package main

import (
	"fmt"
	"os"

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

	p := tea.NewProgram(tui.NewApp(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kubekit: %v\n", err)
		os.Exit(1)
	}
}
