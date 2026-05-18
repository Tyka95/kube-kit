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
			// The leading '█' is a full-block in accent fg painted on the
			// selection bg, so it reads as a bright vertical stripe at the
			// left edge of an unmistakably highlighted row. No floating
			// glyphs — the bar IS part of the bg block, so it can't render
			// disconnected if the user's font drops a rare codepoint.
			plain := " " + label + "   " + detail
			if it.Meta != "" {
				plain += "   " + meta
			}
			// Pad to (Width - 1) to leave room for the leading stripe column.
			plain = padRight(plain, p.Width-1)

			baseBG := theme.SelectionBGAt(p.flashFrame, selectFlashFrames)
			rendered := baseBG.Render(plain)

			if p.flashFrame >= selectFlashFrames {
				// Overlay the shimmer band only after the flash settles.
				width := len(plain)
				start := p.shimmerPos - shimmerWidth/2
				if start < 0 {
					start = 0
				}
				end := start + shimmerWidth
				if end > width {
					end = width
				}
				if end > start {
					bandStyle := lipgloss.NewStyle().
						Background(theme.SelectFlashBG).
						Foreground(theme.Primary).
						Bold(true)
					rendered = baseBG.Render(plain[:start]) +
						bandStyle.Render(plain[start:end]) +
						baseBG.Render(plain[end:])
				}
			}

			// Leading stripe — accent fg on the same selection bg so it
			// joins the row block visually. '█' is U+2588, in any
			// Unicode-capable font.
			stripe := theme.LeftStripe.Render("█")
			row = stripe + rendered
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
