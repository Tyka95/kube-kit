package components

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// Animation: selection-move feedback. The newly-selected row fades from
// bright accent (frame 0) through 4 intermediate steps back to the settled
// SelectBG (frame 5). Each step ~30ms — total ~150ms — smooth to the eye.
const selectFlashFrameInterval = 30 * time.Millisecond
const selectFlashFrames = 5 // frames 1..5; frame 0 is the initial bright paint.

// pickerFlashTickMsg advances the multi-frame fade animation. The token
// guards against late-arriving ticks from a previous move.
type pickerFlashTickMsg struct{ token int }

// Item is a row in the picker. Label is the value returned on selection.
type Item struct {
	Label  string
	Detail string
	Meta   string
}

// Bind is a per-screen custom keybinding (e.g. "r" → "refresh").
type Bind struct {
	Key    string
	Action string
}

// Picker is a Bubble Tea sub-model.
//
// Use New() to construct, Update() / View() like any tea.Model. The picker
// emits typed messages on its parent's Update:
//
//   PickerSelectedMsg{Value string}  — Enter / right-arrow
//   PickerActionMsg{Action string}   — a custom Bind fired
//   PickerCancelMsg                  — Esc / q / left-arrow
//   PickerFilterMsg{Value string}    — '/' filter submitted
//   PickerCommandMsg{Value string}   — ':' command submitted
//   PickerHelpMsg                    — '?'
type Picker struct {
	Title  string
	Items  []Item
	Binds  []Bind
	Width  int
	Height int

	cursor      int
	scroll      int
	filter      string
	filterMode  bool
	commandMode bool
	input       string

	visible []int // indexes into Items after filtering

	// Flash animation state on selection change. flashFrame counts up from 0
	// to selectFlashFrames (settled). Token monotonically increases per move
	// so a late-arriving tick from an earlier move is ignored.
	flashFrame int
	flashToken int
}

// PickerSelectedMsg fires on Enter / right-arrow.
type PickerSelectedMsg struct{ Value string }

// PickerActionMsg fires when a custom Bind key is hit.
type PickerActionMsg struct{ Action string }

// PickerCancelMsg fires on Esc / q / left-arrow with no active filter.
type PickerCancelMsg struct{}

// PickerHelpMsg fires on '?'.
type PickerHelpMsg struct{}

// PickerCommandMsg fires when a ':' command is submitted.
type PickerCommandMsg struct{ Value string }

// New constructs an empty picker. Title is shown via the breadcrumb, not in
// the body.
func New(title string, items []Item, binds []Bind) Picker {
	p := Picker{Title: title, Items: items, Binds: binds}
	p.recomputeVisible()
	return p
}

// SetSize updates the picker's available area.
func (p *Picker) SetSize(width, height int) { p.Width = width; p.Height = height }

// Position returns the current 1-based position of the cursor in the visible list.
func (p Picker) Position() (int, int) {
	if len(p.visible) == 0 {
		return 0, 0
	}
	return p.cursor + 1, len(p.visible)
}

// Init is required by tea.Model.
func (p Picker) Init() tea.Cmd { return nil }

// Update handles key presses + animation ticks.
func (p Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	// Animation tick: advance the fade frame counter and schedule the next
	// tick while frames remain. Stale tokens (from a prior move) are dropped.
	if tick, ok := msg.(pickerFlashTickMsg); ok {
		if tick.token != p.flashToken {
			return p, nil
		}
		if p.flashFrame < selectFlashFrames {
			p.flashFrame++
			if p.flashFrame < selectFlashFrames {
				tok := p.flashToken
				return p, tea.Tick(selectFlashFrameInterval, func(time.Time) tea.Msg {
					return pickerFlashTickMsg{token: tok}
				})
			}
		}
		return p, nil
	}

	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	if p.filterMode || p.commandMode {
		return p.updateInput(km)
	}

	switch km.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		} else if len(p.visible) > 0 {
			p.cursor = len(p.visible) - 1
		}
		p.adjustScroll()
		return p, p.flash()
	case "down", "j":
		if p.cursor < len(p.visible)-1 {
			p.cursor++
		} else {
			p.cursor = 0
		}
		p.adjustScroll()
		return p, p.flash()
	case "enter", "right", "l":
		if len(p.visible) == 0 {
			return p, nil
		}
		val := p.Items[p.visible[p.cursor]].Label
		return p, func() tea.Msg { return PickerSelectedMsg{Value: val} }
	case "esc", "q", "left", "h":
		return p, func() tea.Msg { return PickerCancelMsg{} }
	case "?":
		return p, func() tea.Msg { return PickerHelpMsg{} }
	case "/":
		p.filterMode = true
		p.input = p.filter
		return p, nil
	case ":":
		p.commandMode = true
		p.input = ""
		return p, nil
	}

	// Custom binds.
	for _, b := range p.Binds {
		if b.Key == km.String() {
			action := b.Action
			return p, func() tea.Msg { return PickerActionMsg{Action: action} }
		}
	}

	return p, nil
}

func (p Picker) updateInput(km tea.KeyMsg) (Picker, tea.Cmd) {
	switch km.String() {
	case "esc":
		p.filterMode = false
		p.commandMode = false
		p.input = ""
		return p, nil
	case "enter":
		val := p.input
		wasCmd := p.commandMode
		p.filterMode = false
		p.commandMode = false
		if wasCmd {
			p.input = ""
			return p, func() tea.Msg { return PickerCommandMsg{Value: val} }
		}
		p.filter = val
		p.input = ""
		p.recomputeVisible()
		p.cursor = 0
		p.scroll = 0
		return p, nil
	case "backspace":
		if len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
		}
		return p, nil
	}
	if len(km.Runes) > 0 {
		p.input += string(km.Runes)
	}
	return p, nil
}

// flash starts a multi-frame selection fade. Frame 0 is bright accent;
// each tick advances toward the settled SelectBG. The token guards
// against ticks from a previous (now-stale) move.
func (p *Picker) flash() tea.Cmd {
	p.flashToken++
	p.flashFrame = 0
	tok := p.flashToken
	return tea.Tick(selectFlashFrameInterval, func(time.Time) tea.Msg {
		return pickerFlashTickMsg{token: tok}
	})
}

func (p *Picker) recomputeVisible() {
	p.visible = p.visible[:0]
	if p.filter == "" {
		for i := range p.Items {
			p.visible = append(p.visible, i)
		}
		return
	}
	needle := strings.ToLower(p.filter)
	for i, it := range p.Items {
		if strings.Contains(strings.ToLower(it.Label), needle) ||
			strings.Contains(strings.ToLower(it.Detail), needle) {
			p.visible = append(p.visible, i)
		}
	}
}

func (p *Picker) adjustScroll() {
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	} else if p.cursor >= p.scroll+p.bodyRows() {
		p.scroll = p.cursor - p.bodyRows() + 1
	}
}

func (p Picker) bodyRows() int {
	rows := p.Height
	if p.filterMode || p.commandMode || p.filter != "" {
		rows--
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

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

		labelCol := theme.ListLabel.Render(label)
		detailCol := theme.ListDetail.Render(detail)
		metaCol := theme.ListMeta.Render(meta)

		// 2-char leading column: '▎ ' accent bar on selected row, '  ' otherwise.
		var prefix string
		if isSel {
			prefix = theme.ListAccentBar.Render("▎") + " "
		} else {
			prefix = "  "
		}

		row := prefix + labelCol + "   " + detailCol
		if it.Meta != "" {
			row += "   " + metaCol
		}

		// Pad to full width so the selection bg covers the whole row.
		row = padRight(row, p.Width)
		row = lipgloss.NewStyle().MaxWidth(p.Width).Render(row)

		if isSel {
			row = theme.SelectionBGAt(p.flashFrame, selectFlashFrames).Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	return b.String()
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
