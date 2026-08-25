package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"namegen-lint/lint"
)

// jsonFinding is the wire format for -json output. It carries File
// alongside the fields on lint.Finding, since a single JSON report can
// span multiple input files, and Severity is rendered as text so
// consumers don't need to know the underlying enum.
type jsonFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func main() {
	jsonOutput := flag.Bool("json", false, "report findings as a JSON array on stdout, for CI integration")
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"-"}
	}

	var jsonFindings []jsonFinding
	exitCode := 0
	for _, path := range paths {
		label := path
		var f io.ReadCloser
		if path == "-" {
			label = "<stdin>"
			f = os.Stdin
		} else {
			opened, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
				exitCode = 2
				continue
			}
			f = opened
		}

		findings, err := lint.Lint(f)
		if f != os.Stdin {
			f.Close()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", label, err)
			exitCode = 2
			continue
		}

		for _, fd := range findings {
			if *jsonOutput {
				jsonFindings = append(jsonFindings, jsonFinding{
					File: label, Line: fd.Line, Col: fd.Col, Rule: fd.Rule,
					Severity: fd.Severity.String(), Message: fd.Message,
				})
			} else {
				fmt.Printf("%s:%s\n", label, fd)
			}
			if fd.Severity == lint.Error {
				exitCode = 1
			}
		}
	}

	if *jsonOutput {
		out := jsonFindings
		if out == nil {
			out = []jsonFinding{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "encoding JSON output: %v\n", err)
			os.Exit(2)
		}
	}

	os.Exit(exitCode)
}
