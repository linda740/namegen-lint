// Package lint checks name-list source files used by random name
// generators. The file format and the rules it enforces are described
// in the repository README.
package lint

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxNameLength is the longest an entry name is allowed to be before
// name-too-long fires. It's generous on purpose: legitimate multi-token
// templates like "{first} {middle} {last}, {title}" run long, and this
// rule exists to catch pasted-in garbage (a whole sentence, a stray
// paragraph), not to police normal naming.
const maxNameLength = 80

// Severity ranks how serious a Finding is. Error means the entry will
// misbehave at generation time (skipped, mis-weighted, or producing a
// malformed template). Warning is style only and never affects output.
type Severity int

const (
	Warning Severity = iota
	Error
)

func (s Severity) String() string {
	if s == Error {
		return "error"
	}
	return "warning"
}

// Finding is a single issue reported by a rule, anchored to a line and
// column in the source file. Line and Col are 1-based.
type Finding struct {
	Line     int
	Col      int
	Rule     string
	Severity Severity
	Message  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%d:%d: %s: %s: %s", f.Line, f.Col, f.Severity, f.Rule, f.Message)
}

// Entry is a single non-blank, non-comment line, parsed into the
// pieces a Rule needs. Col is the 1-based column where Name starts
// within Raw.
type Entry struct {
	Line      int
	Raw       string
	Trimmed   string
	Name      string
	WeightStr string
	HasWeight bool
	Col       int
}

// Rule inspects an Entry and returns any Findings it produces there.
// Rules that need to remember something across entries, the way
// duplicate-name remembers every name it has seen, hold that state on
// the Rule value itself; Lint builds a fresh set of rules on every
// call so state never leaks between files.
//
// Rules only ever see entries with a non-empty Name: a line with no
// name is reported as empty-name and nothing else runs against it,
// since checks like duplicate-name would otherwise treat repeated
// blank entries as duplicates of each other.
type Rule interface {
	Check(e Entry) []Finding
}

// defaultRules returns the rules Lint runs against every entry, in
// the order their findings should appear when more than one fires on
// the same line.
func defaultRules() []Rule {
	return []Rule{
		invalidCharRule{},
		nameTooLongRule{},
		unbalancedBracesRule{},
		newDuplicateNameRule(),
		invalidWeightRule{},
	}
}

type invalidCharRule struct{}

func (invalidCharRule) Check(e Entry) []Finding {
	ch, ok := firstInvalidChar(e.Name)
	if !ok {
		return nil
	}
	return []Finding{{
		Line: e.Line, Col: e.Col, Rule: "invalid-char", Severity: Error,
		Message: fmt.Sprintf("name contains invalid character %q", ch),
	}}
}

type nameTooLongRule struct{}

func (nameTooLongRule) Check(e Entry) []Finding {
	n := utf8.RuneCountInString(e.Name)
	if n <= maxNameLength {
		return nil
	}
	return []Finding{{
		Line: e.Line, Col: e.Col, Rule: "name-too-long", Severity: Warning,
		Message: fmt.Sprintf("name is %d characters long, limit is %d", n, maxNameLength),
	}}
}

type unbalancedBracesRule struct{}

func (unbalancedBracesRule) Check(e Entry) []Finding {
	if !bracesUnbalanced(e.Name) {
		return nil
	}
	return []Finding{{
		Line: e.Line, Col: e.Col, Rule: "unbalanced-braces", Severity: Error,
		Message: fmt.Sprintf("template placeholders are unbalanced (%q)", e.Name),
	}}
}

// duplicateNameRule flags a name that has already appeared earlier in
// the same file, comparing case-insensitively. seen maps a normalized
// name to the line it first appeared on.
type duplicateNameRule struct {
	seen map[string]int
}

func newDuplicateNameRule() *duplicateNameRule {
	return &duplicateNameRule{seen: make(map[string]int)}
}

func (r *duplicateNameRule) Check(e Entry) []Finding {
	norm := strings.ToLower(e.Name)
	firstLine, ok := r.seen[norm]
	if !ok {
		r.seen[norm] = e.Line
		return nil
	}
	return []Finding{{
		Line: e.Line, Col: e.Col, Rule: "duplicate-name", Severity: Error,
		Message: fmt.Sprintf("%q duplicates name on line %d", e.Name, firstLine),
	}}
}

type invalidWeightRule struct{}

func (invalidWeightRule) Check(e Entry) []Finding {
	if !e.HasWeight {
		return nil
	}
	n, err := strconv.Atoi(e.WeightStr)
	if err == nil && n >= 1 {
		return nil
	}
	return []Finding{{
		Line: e.Line, Col: e.Col, Rule: "invalid-weight", Severity: Error,
		Message: fmt.Sprintf("weight %q is not a positive integer", e.WeightStr),
	}}
}

// Lint reads a name-list file from r and returns every finding, in the
// order the offending lines appear. A non-nil error means r itself
// could not be read; malformed content is reported as findings, not
// errors.
func Lint(r io.Reader) ([]Finding, error) {
	var findings []Finding
	rules := defaultRules()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if trimmed != raw {
			findings = append(findings, Finding{
				Line: lineNum, Col: 1, Rule: "trailing-whitespace", Severity: Warning,
				Message: "entry has leading or trailing whitespace",
			})
		}

		name, weightStr, hasWeight := splitEntry(trimmed)
		entry := Entry{
			Line: lineNum, Raw: raw, Trimmed: trimmed,
			Name: name, WeightStr: weightStr, HasWeight: hasWeight,
			Col: indexCol(raw, name),
		}

		if name == "" {
			findings = append(findings, Finding{
				Line: lineNum, Col: entry.Col, Rule: "empty-name", Severity: Error,
				Message: "entry has no name",
			})
			continue
		}

		for _, rule := range rules {
			findings = append(findings, rule.Check(entry)...)
		}
	}

	if err := scanner.Err(); err != nil {
		return findings, err
	}
	return findings, nil
}

// splitEntry separates a trimmed line into its name and optional
// weight. hasWeight reports whether a ':' separator was present at
// all, so "Name:" (an empty weight) can be told apart from "Name" (no
// weight given).
func splitEntry(trimmed string) (name, weightStr string, hasWeight bool) {
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

// bracesUnbalanced reports whether s contains a '{'/'}' placeholder
// pair that doesn't close correctly, either because a '}' appears
// before its opening '{' or because a '{' is never closed at all.
func bracesUnbalanced(s string) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return true
			}
		}
	}
	return depth != 0
}

// firstInvalidChar returns the first rune in s that would corrupt the
// generator's output rather than just look odd: an ASCII control
// character, or a Unicode formatting character (zero-width joiners, a
// byte-order mark, soft hyphens, and the like) that renders invisibly
// and almost always got there by an accidental paste, not on purpose.
func firstInvalidChar(s string) (rune, bool) {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return r, true
		}
	}
	return 0, false
}

// indexCol finds the 1-based column where name starts within raw. It
// falls back to column 1 if name is empty or not found verbatim, which
// happens when whitespace around a ':' separator was trimmed away.
func indexCol(raw, name string) int {
	if name == "" {
		return 1
	}
	if i := strings.Index(raw, name); i >= 0 {
		return i + 1
	}
	return 1
}

// knownRules lists every rule name Lint can produce a Finding for.
// Config checks its keys against this set so a typo'd rule name in a
// config file fails loudly instead of being silently ignored.
var knownRules = map[string]bool{
	"duplicate-name":      true,
	"empty-name":          true,
	"invalid-weight":      true,
	"unbalanced-braces":   true,
	"invalid-char":        true,
	"name-too-long":       true,
	"trailing-whitespace": true,
}

// ParseSeverity parses the text form of a Severity, as it appears in
// a config file or on the command line. It's the inverse of
// Severity.String.
func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "error":
		return Error, nil
	case "warning":
		return Warning, nil
	default:
		return 0, fmt.Errorf("unknown severity %q, want \"error\" or \"warning\"", s)
	}
}

// Config overrides the default severity of specific rules, keyed by
// rule name. A value of "off" drops that rule's findings entirely;
// "error" or "warning" changes their severity. Rules left out of the
// map keep the severity Lint gives them.
type Config map[string]string

// Apply filters and re-severities findings according to cfg,
// returning a new slice; the input is left untouched. A non-nil error
// means cfg itself is invalid — an unknown rule name or a severity
// that isn't "error", "warning", or "off" — and the returned slice
// should be discarded.
func (cfg Config) Apply(findings []Finding) ([]Finding, error) {
	if len(cfg) == 0 {
		return findings, nil
	}

	overrides := make(map[string]Severity, len(cfg))
	for rule, value := range cfg {
		if !knownRules[rule] {
			return nil, fmt.Errorf("config: unknown rule %q", rule)
		}
		if value == "off" {
			continue
		}
		sev, err := ParseSeverity(value)
		if err != nil {
			return nil, fmt.Errorf("config: rule %q: %w", rule, err)
		}
		overrides[rule] = sev
	}

	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if cfg[f.Rule] == "off" {
			continue
		}
		if sev, ok := overrides[f.Rule]; ok {
			f.Severity = sev
		}
		out = append(out, f)
	}
	return out, nil
}
