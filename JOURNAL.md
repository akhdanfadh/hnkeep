# Learning by Coding (journal)

Periodically, I document what I learn in this journal.
My learning entries are first written in the code itself with `NOTE:` comment tag.
When it is not relevant anymore, I move it here for future reference.
Most likely when the project is ready for release, I will move them all here.

---

````go
// internal/cli/cli.go (0c5e478fccc7d16002297b176f858fb821105725)
func Run() error {
  ...

 // NOTE: You may see a new blank line appended before the closing backtick
 // eventhough you may not see any newline char in the input files in your IDE.
 // This is how the POSIX standard for text files.
 // You need to use a hex editor to see it (`cat -A <file>` or `od -c <file>`).
 // - https://stackoverflow.com/questions/729692/why-should-text-files-end-with-a-newline
 output := fmt.Sprintf("You are reading the output file. Here is a copy of your input:\n\n```\n%s\n```", input)
 if err := writeOutput(cfg.OutputPath, output); err != nil {
  return fmt.Errorf("writing output: %w", err)
 }

  ...
}
````

---

```go
// internal/cli/cli.go (7cc4f161745b190895323a5033973e8130e6d9f3)
func Run() error {
  ...
 // NOTE: Go strings are immutable, so using string concatenation in a loop
 // can lead to excessive memory allocations (a hint from `go vet`).
 // - https://stackoverflow.com/questions/1760757/how-to-efficiently-concatenate-strings-in-go/47798475#47798475.
 var output strings.Builder
 for _, bm := range bookmarks {
  fmt.Fprintf(&output, "%d %d\n", bm.ID, bm.Timestamp)
 }
  ...
}
```
