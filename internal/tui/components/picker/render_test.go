package picker

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Force lipgloss to emit ANSI sequences even when tests aren't running in
// a real terminal. Without this, all styles render to plain text and the
// test below cannot distinguish styled-but-otherwise-equal outputs from
// truly-undifferentiated ones.
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// stripANSI removes CSI escape sequences from s so callers can scan for
// visible characters without ANSI noise.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestRenderRowRuneSafeAcrossShimmerPositions builds a selected row whose
// label contains a multi-byte UTF-8 character (`·` U+00B7, 2 bytes) and
// asserts the renderer produces valid UTF-8, free of '?' or '�' replacement
// glyphs, at every possible shimmer head position. Catches the byte-slice
// bug that produces `??` mid-row when the band falls on `·`.
func TestRenderRowRuneSafeAcrossShimmerPositions(t *testing.T) {
	items := []Item{
		{Label: "Pods", Detail: "list · logs · shell · inspect"},
	}
	p := New("test", items, nil)
	p.SetSize(80, 10)
	// Pin into resting-selected state so the shimmer code path executes.
	p.flashFrame = selectFlashFrames

	rowText := "  Pods   list · logs · shell · inspect"
	maxPos := len([]rune(rowText)) + 5
	for pos := 0; pos <= maxPos; pos++ {
		p.shimmerPos = pos
		out := p.View()

		if !utf8.ValidString(out) {
			t.Errorf("shimmerPos=%d produced invalid UTF-8: %q", pos, out)
		}
		visible := stripANSI(out)
		if strings.ContainsRune(visible, '�') {
			t.Errorf("shimmerPos=%d produced a U+FFFD replacement character in %q", pos, visible)
		}
		if strings.Contains(visible, "??") {
			t.Errorf("shimmerPos=%d produced literal '??' in %q", pos, visible)
		}
	}
}
