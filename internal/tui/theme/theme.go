// Package theme defines the kubekit color palette and shared lipgloss styles.
// Six semantic tokens map onto Tokyo Night hex values; lipgloss handles the
// 256-color and ANSI-16 fallbacks automatically.
package theme

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color tokens.
var (
	Primary lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#1f2335", Dark: "#e0e0e0"}
	Accent  lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#3d59a1", Dark: "#7aa2f7"}
	Muted   lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#6e7388", Dark: "#565f89"}
	Success lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#587539", Dark: "#9ece6a"}
	Warn    lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#8f5e15", Dark: "#e0af68"}
	Danger  lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#8c4351", Dark: "#f7768e"}

	// SelectBG is the row-selection background.
	SelectBG     lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#c4d4ee", Dark: "#283457"}
	// SelectFlashBG is the brief on-move tint — significantly brighter than
	// SelectBG so the eye registers the change in 150ms.
	SelectFlashBG lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#7aa2f7", Dark: "#7aa2f7"}
)

// Pre-built styles. Use Copy() in callers to derive screen-specific variants.
var (
	Body     = lipgloss.NewStyle().Foreground(Primary)
	Dim      = lipgloss.NewStyle().Foreground(Muted)
	BoldText = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	// Rules are explicitly accent-foreground so they remain visible across
	// terminal themes where C_MUTED can wash out against a dark bg.
	BorderColor   = Accent
	BorderStyle   = lipgloss.NormalBorder()
	HeaderBorder  = lipgloss.NewStyle().Foreground(Accent)
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

	// SelectionMarker is the '❯' caret rendered to the left of the selected
	// row. Accent fg, bold, no background — appears as a floating pointer.
	SelectionMarker = lipgloss.NewStyle().Foreground(Accent).Bold(true)
)

// SelectionBGAt returns the row-selection background color at a given fade
// frame. Frame 0 = full flash (bright accent), frame N (settled) = SelectBG.
// In between the RGB is linearly interpolated so the eye sees a smooth fade.
// `total` should be the number of frames before reaching settled.
func SelectionBGAt(frame, total int) lipgloss.Style {
	if frame <= 0 {
		return lipgloss.NewStyle().Background(SelectFlashBG)
	}
	if frame >= total {
		return lipgloss.NewStyle().Background(SelectBG)
	}
	// Interpolate on dark-theme hexes (kubekit is dark-first).
	r := lerpHex("#7aa2f7", "#283457", float64(frame)/float64(total))
	return lipgloss.NewStyle().Background(lipgloss.Color(r))
}

// lerpHex interpolates between two #rrggbb strings.
func lerpHex(a, b string, t float64) string {
	ar, ag, ab := parseHex(a)
	br, bg, bb := parseHex(b)
	mix := func(x, y int) int { return int(float64(x) + (float64(y)-float64(x))*t) }
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func parseHex(s string) (r, g, b int) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0
	}
	parse := func(hi, lo byte) int {
		return hexNibble(hi)*16 + hexNibble(lo)
	}
	return parse(s[1], s[2]), parse(s[3], s[4]), parse(s[5], s[6])
}

func hexNibble(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	}
	return 0
}

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
