#!/bin/sh
# One repository entry point. Local checks and CI run the same commands, so no environment has
# a private definition of "green".
set -eu
cd "$(dirname "$0")"

usage() {
    echo "usage: ./x [check|fmt|lint|test|baymax|node|shell|guard|audit|dev|hooks] [args...]" >&2
    exit 2
}

command=${1:-check}
if [ "$#" -gt 0 ]; then shift; fi

case "$command" in
    # Everything a pull request must pass, except audit, which needs the network.
    check)
        [ "$#" -eq 0 ] || usage
        ./x fmt --check
        ./x lint
        ./x test
        ./x baymax
        ./x shell
        ./x guard
        ;;
    fmt)
        if [ "${1:-}" = "--check" ]; then
            unformatted=$(gofmt -l .)
            if [ -n "$unformatted" ]; then
                printf 'format these with: gofmt -w %s\n' "$unformatted" >&2
                exit 1
            fi
        else
            gofmt -l -w .
        fi
        ;;
    lint) go vet ./... ;;
    # Extra arguments go to go test, like ./x test ./test -run 'TestCLI/no_arguments'.
    test)
        if [ "$#" -eq 0 ]; then set -- ./...; fi
        go test "$@"
        ;;
    # Baymax owns offline adapter and real-ask integration checks; node is the old alias.
    baymax|node)
        exec node test/baymax/runner.mjs "$@"
        ;;
    shell)
        command -v shellcheck >/dev/null 2>&1 || { echo "shell: shellcheck is not on PATH" >&2; exit 2; }
        shellcheck x install.sh scripts/*.sh .githooks/*
        ;;
    guard) scripts/guard.sh ;;
    # Needs the network for the vulnerability database; CI and releases run it, not pre-commit.
    audit) go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./... ;;
    # Build this checkout and run it: ./x dev runs, ./x dev setup --check.
    dev)
        go build -o "${TMPDIR:-/tmp}/ask-dev" ./cmd/ask
        exec "${TMPDIR:-/tmp}/ask-dev" "$@"
        ;;
    hooks)
        [ "$#" -eq 0 ] || usage
        git config core.hooksPath .githooks
        printf 'git hooks installed: pre-commit runs gofmt and go vet.\n'
        ;;
    *) usage ;;
esac
