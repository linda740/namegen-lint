package lint

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestLint(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []Finding
	}{
		{
			name:  "plain names and a valid weight",
			input: "Alice\nBob:3\n",
			want:  nil,
		},
		{
			name:  "duplicate name differing only by case",
			input: "Alice\nalice\n",
			want: []Finding{
				{Line: 2, Col: 1, Rule: "duplicate-name", Severity: Error,
					Message: `"alice" duplicates name on line 1`},
			},
		},
		{
			name:  "colon with nothing before it",
			input: ":5\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "empty-name", Severity: Error,
					Message: "entry has no name"},
			},
		},
		{
			name:  "colon with nothing after it",
			input: "Carol:\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-weight", Severity: Error,
					Message: `weight "" is not a positive integer`},
			},
		},
		{
			name:  "negative weight",
			input: "Dave:-1\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-weight", Severity: Error,
					Message: `weight "-1" is not a positive integer`},
			},
		},
		{
			name:  "zero weight",
			input: "Eve:0\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-weight", Severity: Error,
					Message: `weight "0" is not a positive integer`},
			},
		},
		{
			name:  "non-numeric weight",
			input: "Frank:abc\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-weight", Severity: Error,
					Message: `weight "abc" is not a positive integer`},
			},
		},
		{
			name:  "extra colon makes the weight field unparseable",
			input: "Gina:2:3\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-weight", Severity: Error,
					Message: `weight "2:3" is not a positive integer`},
			},
		},
		{
			name:  "unclosed placeholder",
			input: "{first\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "unbalanced-braces", Severity: Error,
					Message: `template placeholders are unbalanced ("{first")`},
			},
		},
		{
			name:  "stray closing brace",
			input: "first}\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "unbalanced-braces", Severity: Error,
					Message: `template placeholders are unbalanced ("first}")`},
			},
		},
		{
			name:  "balanced multi-token template",
			input: "{first} {last}\n",
			want:  nil,
		},
		{
			name:  "comments and blank lines are skipped, not counted as entries",
			input: "# first names\n\nAlice\n",
			want:  nil,
		},
		{
			name:  "leading and trailing whitespace",
			input: "  Alice  \n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "trailing-whitespace", Severity: Warning,
					Message: "entry has leading or trailing whitespace"},
			},
		},
		{
			name:  "unicode name is fine on its own",
			input: "Zoë\n",
			want:  nil,
		},
		{
			name:  "whitespace-only line is blank, not an empty name",
			input: "   \n",
			want:  nil,
		},
		{
			name:  "embedded control character",
			input: "Ali\x07ce\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-char", Severity: Error,
					Message: `name contains invalid character '\a'`},
			},
		},
		{
			name:  "zero-width space is a format character, not visible whitespace",
			input: "Ali" + string(rune(0x200b)) + "ce\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "invalid-char", Severity: Error,
					Message: "name contains invalid character " + strconv.QuoteRune(0x200b)},
			},
		},
		{
			name:  "name over the length limit",
			input: strings.Repeat("a", 81) + "\n",
			want: []Finding{
				{Line: 1, Col: 1, Rule: "name-too-long", Severity: Warning,
					Message: "name is 81 characters long, limit is 80"},
			},
		},
		{
			name:  "name at the length limit is fine",
			input: strings.Repeat("a", 80) + "\n",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Lint(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("Lint returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Lint(%q) =\n  %v\nwant\n  %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestConfigApply(t *testing.T) {
	base := []Finding{
		{Line: 1, Col: 1, Rule: "trailing-whitespace", Severity: Warning, Message: "a"},
		{Line: 2, Col: 1, Rule: "name-too-long", Severity: Warning, Message: "b"},
		{Line: 3, Col: 1, Rule: "duplicate-name", Severity: Error, Message: "c"},
	}

	t.Run("empty config leaves findings untouched", func(t *testing.T) {
		got, err := Config{}.Apply(base)
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		if !reflect.DeepEqual(got, base) {
			t.Errorf("Apply(%v) = %v, want unchanged", base, got)
		}
	})

	t.Run("off drops matching findings and leaves others alone", func(t *testing.T) {
		got, err := Config{"trailing-whitespace": "off"}.Apply(base)
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		want := []Finding{base[1], base[2]}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Apply = %v, want %v", got, want)
		}
	})

	t.Run("severity override changes only the named rule", func(t *testing.T) {
		got, err := Config{"name-too-long": "error"}.Apply(base)
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		if got[1].Severity != Error {
			t.Errorf("name-too-long severity = %v, want Error", got[1].Severity)
		}
		if got[0].Severity != Warning || got[2].Severity != Error {
			t.Errorf("unrelated findings changed: %v", got)
		}
	})

	t.Run("unknown rule name is an error", func(t *testing.T) {
		_, err := Config{"not-a-real-rule": "off"}.Apply(base)
		if err == nil {
			t.Fatal("Apply with unknown rule name returned no error")
		}
	})

	t.Run("unrecognized severity value is an error", func(t *testing.T) {
		_, err := Config{"name-too-long": "critical"}.Apply(base)
		if err == nil {
			t.Fatal("Apply with bad severity value returned no error")
		}
	})

	t.Run("input slice is not mutated", func(t *testing.T) {
		input := []Finding{{Line: 1, Col: 1, Rule: "name-too-long", Severity: Warning, Message: "x"}}
		if _, err := (Config{"name-too-long": "error"}).Apply(input); err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		if input[0].Severity != Warning {
			t.Errorf("Apply mutated its input: %v", input)
		}
	})
}
