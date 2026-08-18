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
)

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

// Lint reads a name-list file from r and returns every finding, in the
// order the offending lines appear. A non-nil error means r itself
// could not be read; malformed content is reported as findings, not
// errors.
func Lint(r io.Reader) ([]Finding, error) {
	var findings []Finding
	seen := make(map[string]int) // normalized name -> first line it appeared on

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
		col := indexCol(raw, name)

		if name == "" {
			findings = append(findings, Finding{
				Line: lineNum, Col: col, Rule: "empty-name", Severity: Error,
				Message: "entry has no name",
			})
			continue
		}

		if bracesUnbalanced(name) {
			findings = append(findings, Finding{
				Line: lineNum, Col: col, Rule: "unbalanced-braces", Severity: Error,
				Message: fmt.Sprintf("template placeholders are unbalanced (%q)", name),
			})
		}

		norm := strings.ToLower(name)
		if firstLine, ok := seen[norm]; ok {
			findings = append(findings, Finding{
				Line: lineNum, Col: col, Rule: "duplicate-name", Severity: Error,
				Message: fmt.Sprintf("%q duplicates name on line %d", name, firstLine),
			})
		} else {
			seen[norm] = lineNum
		}

		if hasWeight {
			n, err := strconv.Atoi(weightStr)
			if err != nil || n < 1 {
				findings = append(findings, Finding{
					Line: lineNum, Col: col, Rule: "invalid-weight", Severity: Error,
					Message: fmt.Sprintf("weight %q is not a positive integer", weightStr),
				})
			}
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
