package lint

import (
	"strings"
	"testing"
)

// FuzzLint feeds arbitrary input at Lint looking for two things: a
// panic, and a Finding that points outside the input it supposedly
// came from (a line past the last one, or a column past the end of
// that line). The seeds are drawn from the malformed-input cases in
// lint_test.go plus a few shapes that test isn't covering: CRLF line
// endings and an oversized weight field.
func FuzzLint(f *testing.F) {
	seeds := []string{
		"",
		"\n",
		"Alice\n",
		"Alice\nalice\n",
		"Name:3\n",
		"Name:\n",
		"Name:-1\n",
		"Name:2:3\n",
		":5\n",
		"{first} {last}\n",
		"{first\n",
		"first}\n",
		"{{}}\n",
		"# comment\n\nAlice\n",
		"  Alice  \n",
		"Ali\x07ce\n",
		"Ali" + string(rune(0x200b)) + "ce\n",
		strings.Repeat("a", 200) + "\n",
		"Alice\r\nBob\r\n",
		"Alice:999999999999999999999999999999\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		findings, err := Lint(strings.NewReader(input))
		if err != nil {
			// The only way a string reader makes Lint fail is a line
			// past the scanner's buffer limit; nothing to check then.
			return
		}

		lines := strings.Split(input, "\n")
		for _, fd := range findings {
			if fd.Line < 1 || fd.Line > len(lines) {
				t.Fatalf("finding %+v has line outside input (input has %d lines)", fd, len(lines))
			}
			if fd.Col < 1 {
				t.Fatalf("finding %+v has non-positive column", fd)
			}
			if raw := lines[fd.Line-1]; fd.Col > len(raw)+1 {
				t.Fatalf("finding %+v has column past end of its line %q", fd, raw)
			}
		}
	})
}
