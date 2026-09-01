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
