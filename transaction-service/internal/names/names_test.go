package names

import "testing"

// Invisible and combining characters are written as \U escapes so that no
// editor normalization can silently change what these tests check.

func TestValidateAcceptsVietnameseAndSymbols(t *testing.T) {
	for _, name := range []string{
		"Thanh", "Thành", "Nguyễn Văn A", "Đức_Anh-99", "Bé.Na", "★Hoa♥", "☆♡☆", "ABC", "Một Hai Ba Bốn",
	} {
		if _, problem := Validate(name); problem != "" {
			t.Errorf("%q rejected: %s", name, problem)
		}
	}
}

func TestValidateLengthCountsGraphemes(t *testing.T) {
	// A-06: 2 and 17 graphemes are rejected, 3 and 16 accepted.
	cases := map[string]string{
		"ab":                "too_short",
		"abc":               "",
		"ầầầ":               "",
		"aaaaaaaaaaaaaaaa":  "",
		"aaaaaaaaaaaaaaaaa": "too_long",
		// 16 letters with tone marks are 16 graphemes, not 16+ runes or bytes.
		"ẫẫẫẫẫẫẫẫẫẫẫẫẫẫẫẫ": "",
	}
	for name, want := range cases {
		if _, got := Validate(name); got != want {
			t.Errorf("%q: want %q, got %q", name, want, got)
		}
	}
	// Decomposed: 'a' + U+0302 + U+0300 is one grapheme, so three of them are valid.
	decomposed := "a\U00000302\U00000300a\U00000302\U00000300a\U00000302\U00000300"
	if _, got := Validate(decomposed); got != "" {
		t.Errorf("decomposed ầầầ rejected: %s", got)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]string{
		"abc!":                      ProblemInvalidChar,
		"tên😀":                      ProblemInvalidChar,
		"ñandu":                     ProblemInvalidChar,
		"王小明":                       ProblemInvalidChar,
		"abc\U00000301x":            ProblemInvalidChar, // c + combining acute has no Vietnamese form
		"abc\U0000200Bd":            ProblemInvalidChar, // zero-width space
		"a\tbc":                     ProblemInvalidChar,
		" abc":                      ProblemEdgeSpace,
		"abc ":                      ProblemEdgeSpace,
		"ab  c":                     ProblemRepeatedSpace,
		"♥\U0000FE0Fabc":            ProblemInvalidChar, // emoji variation selector
		"ＡＢＣ":                       ProblemInvalidChar, // full-width letters
		"abc\U000000A0de":           ProblemInvalidChar, // no-break space
		"Tha\U00000300\U00000300nh": ProblemInvalidChar, // stacked marks do not compose
	}
	for name, want := range cases {
		if _, got := Validate(name); got != want {
			t.Errorf("%q: want %q, got %q", name, want, got)
		}
	}
}

func TestKeyIsCaseInsensitiveButAccentSensitive(t *testing.T) {
	a, _ := Validate("Thanh")
	b, _ := Validate("thanh")
	c, _ := Validate("Thành")
	d, _ := Validate("THÀNH")
	if a.Key != b.Key {
		t.Errorf("A-03: Thanh/thanh keys differ: %q %q", a.Key, b.Key)
	}
	if a.Key == c.Key {
		t.Errorf("A-04: Thanh/Thành keys collide: %q", a.Key)
	}
	if c.Key != d.Key {
		t.Errorf("Thành/THÀNH keys differ: %q %q", c.Key, d.Key)
	}
	e, _ := Validate("Đức")
	f, _ := Validate("đức")
	if e.Key != f.Key {
		t.Errorf("Đức/đức keys differ")
	}
}

func TestComposedAndDecomposedAreTheSameName(t *testing.T) {
	// A-05
	composed, p1 := Validate("Thành")
	decomposed, p2 := Validate("Tha\U00000300nh")
	if p1 != "" || p2 != "" {
		t.Fatalf("unexpected problems %q %q", p1, p2)
	}
	if composed.Key != decomposed.Key || composed.Display != decomposed.Display {
		t.Errorf("composed %+v != decomposed %+v", composed, decomposed)
	}
}

func TestSearchKeyStripsAccents(t *testing.T) {
	cases := map[string]string{
		"Thành":           "thanh",
		"Đức Anh":         "duc anh",
		"NGUYỄN":          "nguyen",
		"★Hoa♥":           "★hoa♥",
		"Tha\U00000300nh": "thanh",
	}
	for in, want := range cases {
		if got := SearchKey(in); got != want {
			t.Errorf("SearchKey(%q) = %q, want %q", in, got, want)
		}
	}
}
