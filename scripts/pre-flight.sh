#!/usr/bin/env bash
#
# pre-flight.sh — Run all CI checks locally before pushing.
# Mirrors the GitHub Actions CI pipeline (ci.yml + release.yml).
#
# Usage:
#   ./scripts/pre-flight.sh          # run everything
#   ./scripts/pre-flight.sh --quick  # skip stress tests and lint
#
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
RESET='\033[0m'

pass=0
fail=0
skipped=0

step() {
    printf "\n${BOLD}▶ %s${RESET}\n" "$1"
}

ok() {
    printf "${GREEN}  ✓ %s${RESET}\n" "$1"
    pass=$((pass + 1))
}

fail() {
    printf "${RED}  ✗ %s${RESET}\n" "$1"
    fail=$((fail + 1))
}

skip() {
    printf "${YELLOW}  ⊘ %s (skipped)${RESET}\n" "$1"
    skipped=$((skipped + 1))
}

QUICK=false
if [[ "${1:-}" == "--quick" ]]; then
    QUICK=true
fi

# ─── Go environment ──────────────────────────────────────────────
step "Go environment"
GO_VERSION=$(go version | awk '{print $3}')
printf "  %s\n" "$GO_VERSION"

# ─── Module tidiness ─────────────────────────────────────────────
step "go mod tidy (check for drift)"
cp go.mod go.mod.bak
cp go.sum go.sum.bak 2>/dev/null || true
go mod tidy
if diff -q go.mod go.mod.bak >/dev/null 2>&1; then
    ok "go.mod is tidy"
else
    fail "go.mod changed after 'go mod tidy' — commit the updated go.mod"
fi
mv go.mod.bak go.mod 2>/dev/null || true
mv go.sum.bak go.sum 2>/dev/null || true

# ─── Build ───────────────────────────────────────────────────────
step "go build"
if go build ./... 2>&1; then
    ok "build succeeded"
else
    fail "build failed"
fi

# ─── Cross-compilation (CI runs on Linux) ────────────────────────
step "Cross-compile for CI targets"
# Only build packages that CI sees (exclude local_tests which is gitignored)
CI_PKGS=$(go list ./... 2>/dev/null | grep -v local_tests || true)
cross_ok=true
for target_os in linux darwin windows; do
    if ! GOOS=$target_os GOARCH=amd64 go build $CI_PKGS 2>&1; then
        fail "cross-compile failed for GOOS=$target_os"
        cross_ok=false
    fi
done
if [[ "$cross_ok" == true ]]; then
    ok "cross-compile succeeded (linux, darwin, windows)"
fi

# ─── Vet ─────────────────────────────────────────────────────────
step "go vet"
if go vet ./... 2>&1; then
    ok "vet clean"
else
    fail "vet found issues"
fi

# ─── Tests (race, short) ────────────────────────────────────────
step "go test -race -short ./..."
if go test -race -count=1 -short ./... 2>&1; then
    ok "tests passed (short mode)"
else
    fail "tests failed"
fi

# ─── Tests (full, including stress) ──────────────────────────────
if [[ "$QUICK" == false ]]; then
    step "go test -race ./... (full, including stress tests)"
    if go test -race -count=1 ./... 2>&1; then
        ok "tests passed (full)"
    else
        fail "tests failed (full)"
    fi
else
    skip "full tests (use without --quick to include)"
fi

# ─── Benchmarks (compile check only) ────────────────────────────
step "go test -bench=. -benchtime=1x (compile check)"
if go test -bench=. -benchtime=1x -run='^$' ./... >/dev/null 2>&1; then
    ok "benchmarks compile and run"
else
    fail "benchmarks failed"
fi

# ─── golangci-lint ───────────────────────────────────────────────
if [[ "$QUICK" == false ]]; then
    step "golangci-lint"
    if command -v golangci-lint >/dev/null 2>&1; then
        if golangci-lint run ./... 2>&1; then
            ok "lint clean"
        else
            fail "lint found issues"
        fi
    else
        skip "golangci-lint not installed (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"
    fi
else
    skip "golangci-lint (use without --quick to include)"
fi

# ─── Example tests ───────────────────────────────────────────────
step "Example tests"
EXAMPLE_OUTPUT=$(go test -v -run '^Example' -short ./... 2>&1)
EXAMPLE_PASS=$(echo "$EXAMPLE_OUTPUT" | grep -c "^--- PASS" || true)
EXAMPLE_RUN=$(echo "$EXAMPLE_OUTPUT" | grep -c "^=== RUN" || true)
if [[ "$EXAMPLE_RUN" -gt 0 ]]; then
    ok "$EXAMPLE_RUN examples ran ($EXAMPLE_PASS with output verification)"
else
    fail "no examples found or examples failed"
fi

# ─── CHANGELOG sanity ────────────────────────────────────────────
step "CHANGELOG check"
if [[ -f CHANGELOG.md ]]; then
    if grep -q '## \[0\.' CHANGELOG.md; then
        ok "CHANGELOG.md has version entries"
    else
        fail "CHANGELOG.md missing version entries"
    fi
else
    fail "CHANGELOG.md not found"
fi

# ─── Summary ─────────────────────────────────────────────────────
printf "\n${BOLD}────────────────────────────────────${RESET}\n"
printf "${GREEN}  ✓ %d passed${RESET}" "$pass"
if [[ "$fail" -gt 0 ]]; then
    printf "  ${RED}✗ %d failed${RESET}" "$fail"
fi
if [[ "$skipped" -gt 0 ]]; then
    printf "  ${YELLOW}⊘ %d skipped${RESET}" "$skipped"
fi
printf "\n${BOLD}────────────────────────────────────${RESET}\n"

if [[ "$fail" -gt 0 ]]; then
    printf "\n${RED}${BOLD}PR is NOT ready — fix the failures above.${RESET}\n"
    exit 1
else
    printf "\n${GREEN}${BOLD}PR is ready to push.${RESET}\n"
    exit 0
fi
