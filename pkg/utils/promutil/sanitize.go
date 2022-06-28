package promutil

import "regexp"

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func SanitizeLabel(lbl string) string {
	lbl = invalidChars.ReplaceAllString(lbl, "_")
	if lbl[0] >= '0' && lbl[0] <= '9' {
		lbl = "_" + lbl
	}
	return lbl
}
