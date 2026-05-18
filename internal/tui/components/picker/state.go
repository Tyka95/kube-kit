package picker

import "strings"

// Item is a row in the picker. Label is the value returned on selection;
// Detail and Meta are the two informational columns rendered next to it.
type Item struct {
	Label  string
	Detail string
	Meta   string
}

// Bind is a per-screen custom keybinding. e.g. {Key: "r", Action: "refresh"}
// → pressing 'r' emits PickerActionMsg{Action: "refresh"} for the parent
// screen to handle.
type Bind struct {
	Key    string
	Action string
}

// Position returns the 1-based position of the cursor within the currently
// visible (filtered) item list. Returns (0, 0) when the list is empty.
func (p Picker) Position() (int, int) {
	if len(p.visible) == 0 {
		return 0, 0
	}
	return p.cursor + 1, len(p.visible)
}

// recomputeVisible rebuilds the `visible` index slice from the current
// filter buffer. Matching is case-insensitive substring on Label or Detail.
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
