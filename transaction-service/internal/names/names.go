// Package names validates character names and derives their comparison keys
// (P0-020..025, docs/03 §2).
//
//   - 3..16 visible characters, counted as Unicode extended grapheme clusters.
//   - Vietnamese letters, ASCII letters and digits, the space and _ - . ★ ♥ ☆ ♡.
//   - Unique without regard to case, but accents still count: "Thanh" and
//     "thanh" collide, "Thanh" and "Thành" do not.
//
// Input is normalized to NFC first, so composed and decomposed spellings of
// the same name are the same name. A combining mark that has no precomposed
// Vietnamese form survives NFC and is rejected, which closes the
// "invisible combining mark" route around uniqueness (A-05).
//
// Whitespace at the ends and runs of spaces are rejected rather than silently
// rewritten: the rule is not decided yet (GAP-09) and rejecting can be relaxed
// later without creating names that would then be invalid. [ĐỀ XUẤT]
package names

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	MinGraphemes = 3
	MaxGraphemes = 16
)

// Problem codes, returned to the client in error details.
const (
	ProblemTooShort      = "too_short"
	ProblemTooLong       = "too_long"
	ProblemInvalidChar   = "invalid_char"
	ProblemEdgeSpace     = "edge_space"
	ProblemRepeatedSpace = "repeated_space"
)

const vietnameseLetters = "aàáảãạăằắẳẵặâầấẩẫậbcdđeèéẻẽẹêềếểễệfghiìíỉĩịjklmnoòóỏõọôồốổỗộơờớởỡợpqrstuùúủũụưừứửữựvwxyỳýỷỹỵz"

var allowedLetters = func() map[rune]bool {
	m := map[rune]bool{}
	for _, r := range vietnameseLetters {
		m[r] = true
		m[unicode.ToUpper(r)] = true
	}
	return m
}()

var allowedSymbols = map[rune]bool{' ': true, '_': true, '-': true, '.': true, '★': true, '♥': true, '☆': true, '♡': true}

// Casers and transform chains keep state, so each call builds its own.
func fold(s string) string { return cases.Fold().String(s) }

// Result of a successful validation.
type Name struct {
	Display   string // NFC form, stored as entered
	Key       string // uniqueness key: NFC + case fold, accents kept
	SearchKey string // lower-case, accents and đ stripped; search only
}

// Validate returns the normalized name or the first problem found.
func Validate(raw string) (Name, string) {
	display := norm.NFC.String(raw)
	for _, r := range display {
		if !allowedLetters[r] && !allowedSymbols[r] && !(r >= '0' && r <= '9') {
			return Name{}, ProblemInvalidChar
		}
	}
	if strings.HasPrefix(display, " ") || strings.HasSuffix(display, " ") {
		return Name{}, ProblemEdgeSpace
	}
	if strings.Contains(display, "  ") {
		return Name{}, ProblemRepeatedSpace
	}
	n := uniseg.GraphemeClusterCount(display)
	if n < MinGraphemes {
		return Name{}, ProblemTooShort
	}
	if n > MaxGraphemes {
		return Name{}, ProblemTooLong
	}
	return Name{Display: display, Key: Key(display), SearchKey: SearchKey(display)}, ""
}

// Key is the uniqueness key of an already valid display name.
func Key(display string) string {
	return norm.NFC.String(fold(norm.NFC.String(display)))
}

// SearchKey folds case and removes accents ("Thành" -> "thanh", "Đức" ->
// "duc"). It is used by player search and block-list search, which ignore
// accents (docs/03 §4), never for uniqueness.
func SearchKey(s string) string {
	stripMarks := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(stripMarks, fold(norm.NFC.String(s)))
	if err != nil {
		out = s
	}
	return strings.ReplaceAll(out, "đ", "d")
}
