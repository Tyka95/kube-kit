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

	// SelectBG is the row-selection background. Subtle navy on dark, dim blue on light.
	SelectBG lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#dde6f5", Dark: "#1e2a4e"}
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

	ListSelected = lipgloss.NewStyle().Background(SelectBG)

	FooterHint  = lipgloss.NewStyle().Foreground(Muted)
	FooterKey   = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	FooterCount = lipgloss.NewStyle().Foreground(Muted)

	HelpHeader = lipgloss.NewStyle().Foreground(Accent).Bold(true)
)

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
