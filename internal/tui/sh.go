package tui

import "strings"

// shEscape wraps s in single quotes for safe use inside an sh -c string.
// Embedded single quotes are escaped by closing the quoted segment, inserting
// an escaped quote, and reopening — the classic 'foo'\''bar' trick.
func shEscape(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
