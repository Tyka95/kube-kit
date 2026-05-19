package picker

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// View renders the picker body.
func (p Picker) View() string {
	if p.Width == 0 || p.Height == 0 {
		return ""
	}

	var b strings.Builder

	// Filter/command prompt row when active.
	if p.filterMode {
		b.WriteString(" " + theme.FooterKey.Render("/") + " " + theme.Body.Render(p.input))
		b.WriteString("\n")
	} else if p.commandMode {
		b.WriteString(" " + theme.FooterKey.Render(":") + " " + theme.Body.Render(p.input))
		b.WriteString("\n")
	} else if p.filter != "" {
		b.WriteString(" " + theme.FooterKey.Render("/") + " " + theme.Body.Render(p.filter))
		b.WriteString("\n")
	}

	// Empty list.
	if len(p.visible) == 0 {
		b.WriteString("  " + theme.Dim.Render("no matches"))
		return b.String()
	}

	// Measure column widths from visible items, capped.
	maxLbl, maxDet, maxMeta := 0, 0, 0
	for _, idx := range p.visible {
		it := p.Items[idx]
		if w := lipglossWidth(it.Label); w > maxLbl {
			maxLbl = w
		}
		if w := lipglossWidth(it.Detail); w > maxDet {
			maxDet = w
		}
		if w := lipglossWidth(it.Meta); w > maxMeta {
			maxMeta = w
		}
	}
	if maxLbl > 40 {
		maxLbl = 40
	}
	if maxDet > 60 {
		maxDet = 60
	}
	if maxMeta > 24 {
		maxMeta = 24
	}

	body := p.bodyRows()
	for vis := 0; vis < body; vis++ {
		idx := p.scroll + vis
		if idx >= len(p.visible) {
			break
		}
		it := p.Items[p.visible[idx]]
		isSel := idx == p.cursor

		label := padRight(truncate(it.Label, maxLbl), maxLbl)
		detail := padRight(truncate(it.Detail, maxDet), maxDet)
		meta := padLeft(truncate(it.Meta, maxMeta), maxMeta)

		var row string
		if isSel {
			// Selected row layout:
			//   █ <label>   <detail>   <meta>
			// Rendered rune-by-rune so multi-byte UTF-8 characters (like '·')
			// never get sliced in half. The shimmer is foreground-only:
			// per-cell brightness derived from distance to shimmerPos.
			plain := " " + label + "   " + detail
			if it.Meta != "" {
				plain += "   " + meta
			}
			plain = padRight(plain, p.Width-1) // -1 leaves room for left stripe

			baseBG := theme.SelectionBGAt(p.flashFrame, selectFlashFrames)
			labelEnd := 1 + lipglossWidth(label) // first col after the leading space + label

			runes := []rune(plain)
			var rb strings.Builder
			for col, r := range runes {
				// Base fg: bold primary in the label zone, muted elsewhere.
				var base lipgloss.Style
				if col <= labelEnd {
					base = theme.ListLabel
				} else {
					base = theme.ListDetail
				}
				// Apply shimmer glow only after the flash fade has settled.
				var fg lipgloss.Style
				if p.flashFrame >= selectFlashFrames {
					fg = theme.ShimmerGlowAt(col-p.shimmerPos, base)
				} else {
					fg = base
				}
				// Compose: fg style + bg style. Inherit puts fg on top of bg
				// so we never lose the selection background.
				cell := fg.Inherit(baseBG).Render(string(r))
				rb.WriteString(cell)
			}

			stripe := theme.LeftStripe.Render("█")
			row = stripe + rb.String()
		} else {
			// Unselected row: 2-space lead-in matching the marker width so
			// the columns line up between selected and unselected rows.
			labelCol := theme.ListLabel.Render(label)
			detailCol := theme.ListDetail.Render(detail)
			metaCol := theme.ListMeta.Render(meta)
			row = "  " + labelCol + "   " + detailCol
			if it.Meta != "" {
				row += "   " + metaCol
			}
			row = padRight(row, p.Width)
			row = lipgloss.NewStyle().MaxWidth(p.Width).Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	return b.String()
}

// lipglossWidth returns the printable cell width of a styled string by
// stripping ANSI sequences manually. Duplicated from components/footer.go
// because picker is now a separate package; the helper is too tiny to
// justify a shared "stringwidth" package.
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

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipglossWidth(s) <= max {
		return s
	}
	r := []rune(s)
	if max > len(r) {
		max = len(r)
	}
	return string(r[:max])
}

func padRight(s string, w int) string {
	gap := w - lipglossWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func padLeft(s string, w int) string {
	gap := w - lipglossWidth(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}
