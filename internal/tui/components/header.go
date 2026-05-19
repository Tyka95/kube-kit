// Package components renders the persistent chrome (header, breadcrumb,
// footer/action-bar) for every screen.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// HeaderHeight is the fixed number of rows the header occupies.
// Row 1: top rule. Row 2: status segments. Row 3: hints. Row 4: bottom rule.
const HeaderHeight = 4

// Header renders the top chrome: top rule, status segments, hints, bottom rule.
func Header(s state.AppState) string {
	w := s.Width
	if w < 20 {
		w = 20
	}

	rule := theme.HeaderBorder.Render(strings.Repeat("─", w))

	// Status row: kubekit · k8s <ctx> · ns <ns> · aws <profile> <glyph> <detail>
	sep := theme.HeaderSegment.Render("·")
	parts := []string{
		" " + theme.HeaderValue.Render("kubekit"),
	}

	if s.KubeContext != "" {
		parts = append(parts, theme.HeaderSegment.Render("k8s")+" "+theme.HeaderValue.Render(s.KubeContext))
	} else {
		parts = append(parts, theme.HeaderSegment.Render("k8s")+" "+theme.StatusErr.Render("no cluster"))
	}

	ns := s.KubeNamespace
	if ns == "" {
		ns = "default"
	}
	parts = append(parts, theme.HeaderSegment.Render("ns")+" "+theme.HeaderValue.Render(ns))

	if s.AWSProfile != "" || s.AWSStatus != "" {
		aws := theme.HeaderSegment.Render("aws") + " " + theme.HeaderValue.Render(orPlaceholder(s.AWSProfile, "<none>"))
		aws += " " + awsGlyphAndDetail(s)
		parts = append(parts, aws)
	}

	status := strings.Join(parts, "  "+sep+"  ")

	// Hints row.
	var hints []string
	for _, h := range s.KeyHints {
		hints = append(hints, theme.FooterKey.Render(h.Key)+" "+theme.FooterHint.Render(h.Action))
	}
	hintsLine := " " + strings.Join(hints, "  ")
	if len(hints) == 0 {
		hintsLine = ""
	}

	return strings.Join([]string{rule, status, hintsLine, rule}, "\n")
}

func awsGlyphAndDetail(s state.AppState) string {
	switch s.AWSStatus {
	case state.AWSOK:
		if s.AWSCtxAccount != "" && s.AWSAccount != "" && s.AWSCtxAccount != s.AWSAccount {
			return theme.Glyph("warn") + " " + theme.StatusWarn.Render("mismatch ⟶ "+s.AWSCtxAccount)
		}
		return theme.Glyph("ok") + " " + theme.Dim.Render(s.AWSAccount)
	case state.AWSExpired:
		return theme.Glyph("expired") + " " + theme.StatusErr.Render("expired")
	case state.AWSNoAWS:
		return theme.Glyph("no-aws") + " " + theme.Dim.Render("no aws")
	default:
		return theme.Glyph("") + " " + theme.Dim.Render("validating")
	}
}

func orPlaceholder(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// Breadcrumb renders 'Main › X › Y' on its own row.
func Breadcrumb(s state.AppState) string {
	chunks := []string{theme.Breadcrumb.Render("Main")}
	for _, b := range s.Breadcrumbs {
		chunks = append(chunks, theme.Breadcrumb.Render("›"), theme.BreadcrumbHere.Render(b))
	}
	return " " + lipgloss.JoinHorizontal(lipgloss.Left, chunks...)
}
