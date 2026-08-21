package main

import (
	"fmt"
	"io"
	"os"

	"namegen-lint/lint"
)

func main() {
	paths := os.Args[1:]
	if len(paths) == 0 {
		paths = []string{"-"}
	}

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
			fmt.Printf("%s:%s\n", label, fd)
			if fd.Severity == lint.Error {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}
