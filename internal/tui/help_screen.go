package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// HelpScreen renders the global help overlay. It captures the calling screen's
// KeyHints in its constructor so they can be displayed even though the
// app's KeyHints have been replaced.
type HelpScreen struct {
	parentHints []state.KeyHint
}

// NewHelpScreen constructs a HelpScreen that shows parentHints under "Screen keys".
func NewHelpScreen(parentHints []state.KeyHint) *HelpScreen {
	return &HelpScreen{parentHints: parentHints}
}

func (h *HelpScreen) Init() tea.Cmd { return nil }

func (h *HelpScreen) Breadcrumb() string { return "Help" }

func (h *HelpScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{{Key: "esc", Action: "back"}}
}

// Update pops the screen on any key press by returning (nil, nil).
func (h *HelpScreen) Update(msg tea.Msg, _ *App) (Screen, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return nil, nil
	}
	return h, nil
}

// View renders the help body.
func (h *HelpScreen) View(_ *App) string {
	var sb strings.Builder

	// Title
	sb.WriteString("  ")
	sb.WriteString(theme.HelpHeader.Render("KubeKit — help"))
	sb.WriteString("\n\n")

	// Global keys section
	sb.WriteString("  ")
	sb.WriteString(theme.HelpHeader.Render("Global keys"))
	sb.WriteString("\n")

	globalKeys := []struct{ key, action string }{
		{"↑↓", "move selection"},
		{"⏎", "activate"},
		{"esc", "back / cancel"},
		{"/", "filter list"},
		{":", "command palette"},
		{"q", "back (or quit at root)"},
		{"?", "this help"},
	}

	for _, k := range globalKeys {
		sb.WriteString(fmt.Sprintf("    %s  %s\n",
			theme.FooterKey.Render(fmt.Sprintf("%-8s", k.key)),
			theme.Dim.Render(k.action),
		))
	}

	// Screen keys section (only if parent has hints)
	if len(h.parentHints) > 0 {
		sb.WriteString("\n")
		sb.WriteString("  ")
		sb.WriteString(theme.HelpHeader.Render("Screen keys"))
		sb.WriteString("\n")

		for _, hint := range h.parentHints {
			sb.WriteString(fmt.Sprintf("    %s  %s\n",
				theme.FooterKey.Render(fmt.Sprintf("%-8s", hint.Key)),
				theme.Dim.Render(hint.Action),
			))
		}
	}

	return sb.String()
}
