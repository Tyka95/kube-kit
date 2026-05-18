package picker

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleKey dispatches a tea.KeyMsg into state mutations + optional command.
// Pure relative to time: no timers, no I/O, no rendering. Picker is taken
// by value so callers control commit.
func (p Picker) handleKey(km tea.KeyMsg) (Picker, tea.Cmd) {
	if p.filterMode || p.commandMode {
		return p.handleInputModeKey(km)
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

	for _, b := range p.Binds {
		if b.Key == km.String() {
			action := b.Action
			return p, func() tea.Msg { return PickerActionMsg{Action: action} }
		}
	}
	return p, nil
}

// handleInputModeKey handles characters typed into the '/' filter or ':'
// command input mode.
func (p Picker) handleInputModeKey(km tea.KeyMsg) (Picker, tea.Cmd) {
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

// adjustScroll shifts the visible window so the cursor stays in view.
func (p *Picker) adjustScroll() {
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	} else if p.cursor >= p.scroll+p.bodyRows() {
		p.scroll = p.cursor - p.bodyRows() + 1
	}
}

// bodyRows returns how many list rows fit between the filter prompt (if
// any) and the bottom of the picker's reserved area.
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
