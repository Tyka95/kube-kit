// Package theme defines the kubekit color palette and shared lipgloss styles.
// Six semantic tokens map onto the Nord palette (https://www.nordtheme.com/) —
// desaturated, calm tones designed for long-session work next to logs and
// kubectl output. Lipgloss handles 256-color and ANSI-16 fallbacks
// automatically; AdaptiveColor pairs light/dark variants.
package theme

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color tokens — Nord palette.
//
// Dark variants are the Nord defaults (snow-storm + frost + aurora). Light
// variants invert primary/muted and use the slightly darker Frost tones so
// the AdaptiveColor pair stays readable on bright terminal themes.
var (
	Primary lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#2e3440", Dark: "#d8dee9"} // nord4 / nord0
	Accent  lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"} // nord10 / nord8 (frost)
	Muted   lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#4c566a"} // nord3
	Success lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#a3be8c", Dark: "#a3be8c"} // nord14 (aurora green)
	Warn    lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#d08770", Dark: "#ebcb8b"} // nord12 / nord13
	Danger  lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#bf616a", Dark: "#bf616a"} // nord11

	// SelectBG is the row-selection background. Nord polar-night #3b4252 —
	// quiet enough not to fight kubectl output, distinct enough to read as
	// "highlighted" on a #2e3440 terminal background.
	SelectBG lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#e5e9f0", Dark: "#3b4252"} // nord1
	// SelectFlashBG is the brief on-move tint — Nord frost (#5e81ac) so the
	// 150ms flash on move is cool-blue, not a saturated pop.
	SelectFlashBG lipgloss.AdaptiveColor = lipgloss.AdaptiveColor{Light: "#81a1c1", Dark: "#5e81ac"} // nord9 / nord10
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

	// LeftStripe is the bright vertical stripe rendered on the very left
	// edge of the selected row. Accent fg ON the selection bg so it reads
	// as a solid bright bar continuing through the row's highlight block.
	LeftStripe = lipgloss.NewStyle().Foreground(Accent).Background(SelectBG).Bold(true)
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
	// Nord: frost #5e81ac (flash) → polar-night #3b4252 (settled).
	r := lerpHex("#5e81ac", "#3b4252", float64(frame)/float64(total))
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

// ShimmerGlowAt returns the foreground style for a row character at signed
// distance d from the shimmer head:
//
//	d == 0   → bright primary + bold (the moving "head" of the glow)
//	|d| == 1 → shoulder blend (bold, accent-tinted)
//	|d| >= 2 → returns base unchanged (cold cell — no glow effect)
//
// The selection background is owned by the row renderer; this helper only
// shapes the foreground. Same input always produces the same Style — pure.
func ShimmerGlowAt(d int, base lipgloss.Style) lipgloss.Style {
	abs := d
	if abs < 0 {
		abs = -abs
	}
	switch abs {
	case 0:
		return lipgloss.NewStyle().Foreground(Primary).Bold(true)
	case 1:
		return lipgloss.NewStyle().Foreground(Accent).Bold(true)
	default:
		return base
	}
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
