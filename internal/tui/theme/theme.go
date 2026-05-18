// Package theme defines the kubekit color palette and shared lipgloss styles.
// Six semantic tokens map onto Tokyo Night hex values; lipgloss handles the
// 256-color and ANSI-16 fallbacks automatically.
package theme

import "github.com/charmbracelet/lipgloss"

// Color tokens.
var (
	Primary lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#1f2335", Dark: "#e0e0e0"}
	Accent  lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#3d59a1", Dark: "#7aa2f7"}
	Muted   lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#6e7388", Dark: "#565f89"}
	Success lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#587539", Dark: "#9ece6a"}
	Warn    lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#8f5e15", Dark: "#e0af68"}
	Danger  lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#8c4351", Dark: "#f7768e"}

	// SelectBG is the row-selection background. Tokyo Night highlight tone —
	// more saturated than the previous dim navy so the selection actually pops.
	SelectBG     lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#c4d4ee", Dark: "#283457"}
	SelectFlashBG lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#9bb4dc", Dark: "#3d4a7a"}
)

// Pre-built styles. Use Copy() in callers to derive screen-specific variants.
var (
	Body     = lipgloss.NewStyle().Foreground(Primary)
	Dim      = lipgloss.NewStyle().Foreground(Muted)
	BoldText = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	BorderColor   = Muted
	BorderStyle   = lipgloss.NormalBorder()
	HeaderBorder  = lipgloss.NewStyle().Foreground(Muted)
	HeaderSegment = lipgloss.NewStyle().Foreground(Muted)
	HeaderValue   = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	Breadcrumb     = lipgloss.NewStyle().Foreground(Muted)
	BreadcrumbHere = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	StatusOk     = lipgloss.NewStyle().Foreground(Success)
	StatusWarn   = lipgloss.NewStyle().Foreground(Warn)
	StatusErr    = lipgloss.NewStyle().Foreground(Danger)
	StatusUnknown = lipgloss.NewStyle().Foreground(Muted)

	ListLabel   = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	ListDetail  = lipgloss.NewStyle().Foreground(Muted)
	ListMeta    = lipgloss.NewStyle().Foreground(Muted).Italic(true)

	ListSelected      = lipgloss.NewStyle().Background(SelectBG)
	ListSelectedFlash = lipgloss.NewStyle().Background(SelectFlashBG)
	ListAccentBar     = lipgloss.NewStyle().Foreground(Accent).Bold(true)

	FooterHint  = lipgloss.NewStyle().Foreground(Muted)
	FooterKey   = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	FooterCount = lipgloss.NewStyle().Foreground(Muted)

	HelpHeader = lipgloss.NewStyle().Foreground(Accent).Bold(true)
)

// InfoCallout renders a leading-icon callout suitable for transient status
// lines like "X: not yet implemented" or "tunnel active: ...".
//   kind: "info" | "warn" | "error" | "ok"
func InfoCallout(kind, msg string) string {
	var icon, color lipgloss.Style
	switch kind {
	case "ok":
		icon = StatusOk
		color = lipgloss.NewStyle().Foreground(Success)
	case "warn":
		icon = StatusWarn
		color = lipgloss.NewStyle().Foreground(Warn)
	case "error":
		icon = StatusErr
		color = lipgloss.NewStyle().Foreground(Danger)
	default:
		icon = lipgloss.NewStyle().Foreground(Accent)
		color = Body
	}
	glyph := map[string]string{"ok": "✓", "warn": "▲", "error": "✗", "info": "›"}[kind]
	if glyph == "" {
		glyph = "›"
	}
	return "  " + icon.Render(glyph) + "  " + color.Render(msg)
}

// StatusGlyphs returned by Glyph() for each AWS session status code.
func Glyph(status string) string {
	switch status {
	case "ok":
		return StatusOk.Render("✓")
	case "warn", "mismatch":
		return StatusWarn.Render("⚠")
	case "expired":
		return StatusErr.Render("✗")
	case "no-aws":
		return StatusUnknown.Render("–")
	default:
		return StatusUnknown.Render("…")
	}
}
