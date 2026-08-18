package main

import (
	"fmt"
	"os"

	"namegen-lint/lint"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: namegen-lint <file> [file...]")
		os.Exit(2)
	}

	exitCode := 0
	for _, path := range os.Args[1:] {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			exitCode = 2
			continue
		}

		findings, err := lint.Lint(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			exitCode = 2
			continue
		}

		for _, fd := range findings {
			fmt.Printf("%s:%s\n", path, fd)
			if fd.Severity == lint.Error {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}
