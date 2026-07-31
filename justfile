out := if os() == "windows" { "build/dir-age.exe" } else { "build/dir-age" }

# build the binary for the current platform
build:
    go build -o {{out}} .

# build then run, forwarding args, e.g. `just run .`
run *args: build
    ./{{out}} {{args}}

# run the test suite
test:
    go test ./...

# check code formatting (fails if any files need gofmt, or don't parse)
fmt:
    test -z "$(gofmt -l . 2>&1)"

# run go vet
vet:
    go vet ./...

# run fmt, vet, and the test suite - use this before committing
check: fmt vet test
