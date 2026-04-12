package textutil

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var titleCaser = cases.Title(language.English)

// TitleCase converts a string to title case using English locale rules.
func TitleCase(s string) string {
	return titleCaser.String(s)
}
