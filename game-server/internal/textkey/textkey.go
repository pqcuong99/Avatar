// Package textkey derives the accent- and case-insensitive search key used by
// player search and block-list search (docs/03 §4). It must match
// transaction-service/internal/names.SearchKey, which fills
// characters.search_key; it is never used for name uniqueness.
package textkey

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func Search(s string) string {
	strip := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(strip, cases.Fold().String(norm.NFC.String(s)))
	if err != nil {
		out = s
	}
	return strings.TrimSpace(strings.ReplaceAll(out, "đ", "d"))
}
