package theme

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	// Force TrueColor so lipgloss emits ANSI escape codes even outside a TTY.
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestShimmerGlowAtVaryingDistance(t *testing.T) {
	base := Dim
	cases := []struct {
		name string
		d    int
	}{
		{"head", 0},
		{"shoulder-pos", 1},
		{"shoulder-neg", -1},
		{"cold-far", 5},
		{"cold-neg", -5},
	}
	rendered := make(map[string]string, len(cases))
	for _, c := range cases {
		rendered[c.name] = ShimmerGlowAt(c.d, base).Render("X")
	}

	// Head must differ from a far/cold cell.
	if rendered["head"] == rendered["cold-far"] {
		t.Errorf("ShimmerGlowAt(0) and ShimmerGlowAt(5) produced identical output: %q", rendered["head"])
	}
	// Shoulders must differ from the head AND from cold cells.
	if rendered["shoulder-pos"] == rendered["head"] {
		t.Errorf("ShimmerGlowAt(1) and ShimmerGlowAt(0) produced identical output; the shoulder should be distinguishable")
	}
	if rendered["shoulder-pos"] == rendered["cold-far"] {
		t.Errorf("ShimmerGlowAt(1) and ShimmerGlowAt(5) produced identical output")
	}
	// Far-cold cells should fall back to the base style regardless of sign.
	if rendered["cold-far"] != rendered["cold-neg"] {
		t.Errorf("ShimmerGlowAt(5) and ShimmerGlowAt(-5) differ; both should fall back to base")
	}
	// Sanity: every rendering contains the literal payload, not garbled.
	for name, out := range rendered {
		if !strings.Contains(out, "X") {
			t.Errorf("ShimmerGlowAt(%s) dropped the payload: %q", name, out)
		}
	}
}
