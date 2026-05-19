package components

import (
	"fmt"
	"strings"

	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// FooterHeight is the fixed number of rows the footer occupies (top rule + action bar + bottom rule).
const FooterHeight = 3

// FooterOpts allows callers to override the action-bar middle line for special
// modes like pause ("press any key…") without re-implementing the chrome.
type FooterOpts struct {
	Position string // e.g. "3 of 7"; empty hides it
	Override string // when set, replaces the entire action-bar middle line
}

// Footer renders the bottom chrome.
func Footer(s state.AppState, opts FooterOpts) string {
	w := s.Width
	if w < 20 {
		w = 20
	}
	rule := theme.HeaderBorder.Render(strings.Repeat("─", w))

	var middle string
	if opts.Override != "" {
		middle = " " + opts.Override
	} else {
		left := theme.FooterHint.Render("/") + " filter   " +
			theme.FooterHint.Render(":") + " command"

		right := ""
		if opts.Position != "" {
			right += theme.FooterCount.Render(opts.Position) + "   "
		}
		right += theme.FooterHint.Render("↑↓") + " select  " +
			theme.FooterHint.Render("⏎") + " go  " +
			theme.FooterHint.Render("?") + " help"

		pad := w - 2 - lipglossWidth(left) - lipglossWidth(right)
		if pad < 1 {
			pad = 1
		}
		middle = " " + left + strings.Repeat(" ", pad) + right
	}

	return strings.Join([]string{rule, middle, rule}, "\n")
}

// lipglossWidth returns the printable cell width of a styled string by
// stripping ANSI sequences manually. Lipgloss's lipgloss.Width counts cells
// correctly but allocates; this is a hot path so we inline.
func lipglossWidth(s string) int {
	n := 0
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
		n++
	}
	return n
}

// PressAnyKeyOverride returns a footer override for pause() prompts.
func PressAnyKeyOverride() string {
	return theme.Dim.Render("press any key to continue…")
}

// MakePosition formats "n of m" for the picker's position counter.
func MakePosition(n, m int) string {
	if m == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", n, m)
}
