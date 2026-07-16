GO_VERSION := 1.26
CMD := ./cmd/dependency-diff-notes

.PHONY: build dist run clean test coverage build-linux-x86 build-macos-arm64

build:
	go build -o dist/dependency-diff-notes $(CMD)

dist: build-linux-x86 build-macos-arm64

build-linux-x86:
	GOOS=linux GOARCH=amd64 go build -o dist/dependency-diff-notes-linux-x86 $(CMD)

build-macos-arm64:
	GOOS=darwin GOARCH=arm64 go build -o dist/dependency-diff-notes-macos-arm64 $(CMD)

run:
	go run $(CMD)

clean:
	rm -rf coverage/ dist/

test:
	go test ./...

coverage:
	@mkdir -p coverage
	go test -v -coverprofile=coverage/coverage.out -covermode=atomic ./... | tee coverage/test-output.log
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@command -v go-junit-report >/dev/null 2>&1 || { echo "Installing go-junit-report..."; go install github.com/jstemmer/go-junit-report/v2@latest; }
	@GOBIN=$$(go env GOPATH)/bin; cat coverage/test-output.log | $$GOBIN/go-junit-report -set-exit-code > coverage/junit-report.xml
	@echo ""
	@echo "Coverage reports generated:"
	@echo "  - HTML: coverage/coverage.html"
	@echo "  - JUnit XML: coverage/junit-report.xml"
	@echo "  - Coverage profile: coverage/coverage.out"
