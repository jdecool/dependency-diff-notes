# Tests

- **Table-driven**: structure tests as a slice of `{name, input, want}` cases iterated with `t.Run(tt.name, ...)`.
- **Location**: `*_test.go` files next to the tested code, same package (no separate `_test` package unless explicitly needed to avoid an import cycle).
- **Fixtures**: large or shared test data in a package-local `testdata/` directory.
- **Isolation**: tests that touch the filesystem or run external commands use `t.TempDir()` and check the tool's availability (`exec.LookPath`) to `t.Skip()` cleanly if absent.
- **Coverage**: `make coverage` generates an HTML report and a JUnit report in `coverage/`.
  Aim for useful coverage of the business logic, not an arbitrary percentage.
- Before committing: `go test ./...` must pass, `gofmt -l .` must list nothing.
