# namegen-lint

Random name generators are usually fed by a plain text file someone
edited by hand: a list of first names, surnames, or template lines like
`{first} {last}`, sometimes with weights to make certain names show up
more often. Nothing checks that file. A duplicate entry silently doubles
that name's odds. A weight that isn't a number gets ignored by whatever
naive `strconv.Atoi` is parsing it. A stray `{` in a template either
panics the generator or leaks a literal brace into the output. None of
this shows up until someone notices the generator's output looks wrong.

`namegen-lint` reads those files and reports the problem with a line
number, the way a compiler would, instead of leaving it for someone to
spot in the generated output.

## file format

One entry per line.

- `Name` — a plain entry, weight defaults to 1.
- `Name:Weight` — Weight must be a positive integer.
- `{token} {token}` — templates mix literal text with `{placeholder}`
  tokens; braces must balance.
- `# ...` — comment, ignored.
- blank lines are ignored.

## usage

```
$ cat heroes.txt
# fantasy first names
Alice
Bob:3
alice
Carol:
{first} {last

$ go run . heroes.txt
heroes.txt:4:1: error: duplicate-name: "alice" duplicates name on line 2
heroes.txt:5:1: error: invalid-weight: weight "" is not a positive integer
heroes.txt:6:1: error: unbalanced-braces: template placeholders are unbalanced ("{first} {last")
$ echo $?
1
```

Exit status is 1 if any error-level finding was reported, 2 if a file
couldn't be read, 0 otherwise. Warning-level findings (currently just
`trailing-whitespace`) are printed but don't affect the exit status.

Pass `-` (or no arguments at all) to read from stdin instead of a file:

```
$ cat heroes.txt | go run . -
<stdin>:4:1: error: duplicate-name: "alice" duplicates name on line 2
```

## rules

| rule | severity | meaning |
|---|---|---|
| `duplicate-name` | error | same name (case-insensitive) appears more than once |
| `empty-name` | error | entry has no name, e.g. a line that is just `:5` |
| `invalid-weight` | error | the part after `:` isn't a positive integer |
| `unbalanced-braces` | error | `{` and `}` in the entry don't match up |
| `trailing-whitespace` | warning | entry has leading or trailing whitespace |

## install

```
go install .
```

Requires Go 1.22 and nothing else — no third-party dependencies.

## library use

The checks live in `lint` and don't depend on the CLI:

```go
findings, err := lint.Lint(r) // r is an io.Reader
```

`Finding` carries `Line`, `Col`, `Rule`, `Severity`, and `Message`, so a
caller can format it however it wants instead of using `Finding.String`.
