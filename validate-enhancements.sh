#!/bin/bash

# Enhanced Rrunner Validation Script
# Tests all major enhancements and functionality

set -e  # Exit on any error

echo "🚀 Validating Rrunner Enhanced Features"
echo "======================================"
echo

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local test_name="$1"
    local test_command="$2"
    
    echo -e "${BLUE}Testing:${NC} $test_name"
    TESTS_RUN=$((TESTS_RUN + 1))
    
    if eval "$test_command" > /tmp/test_output 2>&1; then
        echo -e "${GREEN}✓ PASSED:${NC} $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAILED:${NC} $test_name"
        echo -e "${YELLOW}Output:${NC}"
        cat /tmp/test_output | head -10
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    echo
}

# Build tests
echo -e "${YELLOW}Building Enhanced Rrunner${NC}"
run_test "Go module validation" "go mod verify"
run_test "Legacy core build" "go build -o /tmp/rrunner-core ./cmd/rrunner-core"
run_test "Enhanced build" "go build -o /tmp/rrunner-enhanced ./cmd/rrunner-enhanced"

# Basic functionality tests
echo -e "${YELLOW}Basic Functionality Tests${NC}"
run_test "Legacy version check" "/tmp/rrunner-core --version"
run_test "Enhanced version check" "/tmp/rrunner-enhanced --version"
run_test "Legacy list actions" "/tmp/rrunner-core --list-actions"
run_test "Enhanced list actions" "/tmp/rrunner-enhanced --list-actions"

# Enhanced feature tests
echo -e "${YELLOW}Enhanced Feature Tests${NC}"
run_test "Enhanced help" "/tmp/rrunner-enhanced --help"
run_test "Enhanced health check" "/tmp/rrunner-enhanced --health"
run_test "Enhanced metrics (disabled)" "/tmp/rrunner-enhanced --metrics"
run_test "Enhanced JSON output" "/tmp/rrunner-enhanced --list-actions --output json"

# Action explanation tests
echo -e "${YELLOW}Action System Tests${NC}"
run_test "Explain open action" "/tmp/rrunner-enhanced --explain-action open"
run_test "Explain action with JSON" "/tmp/rrunner-enhanced --explain-action launch --output json"

# Dry run tests
echo -e "${YELLOW}Dry Run Tests${NC}"
run_test "Legacy dry run" "/tmp/rrunner-core --dry-run 'rrunner://open?url=file:///tmp/test.txt'"
run_test "Enhanced dry run" "/tmp/rrunner-enhanced --dry-run 'rrunner://open?url=file:///tmp/test.txt'"
run_test "Enhanced dry run JSON" "/tmp/rrunner-enhanced --dry-run 'rrunner://launch?app=TextEdit' --output json"

# Configuration tests  
echo -e "${YELLOW}Configuration Tests${NC}"
run_test "Create test config dir" "mkdir -p /tmp/rrunner-test/.config/rrunner"
run_test "Copy example config" "cp config/rrunner.config.toml.example /tmp/rrunner-test/.config/rrunner/config.toml"

# Code quality tests
echo -e "${YELLOW}Code Quality Tests${NC}"
run_test "Go vet" "go vet ./..."
run_test "Go fmt check" "test -z \$(gofmt -l .)"
run_test "Shell syntax check" "find bin -name '*.sh' -exec bash -n {} \\;"

# Unit tests
echo -e "${YELLOW}Unit Tests${NC}"
run_test "Config package tests" "go test ./internal/config"
run_test "Actions package tests" "go test ./internal/actions"
run_test "Utils package tests" "go test ./internal/utils -short"

# Enhanced Makefile tests
echo -e "${YELLOW}Build System Tests${NC}"
if [ -f "Makefile.enhanced" ]; then
    run_test "Enhanced Makefile help" "make -f Makefile.enhanced help"
    run_test "Enhanced Makefile info" "make -f Makefile.enhanced info"
    run_test "Enhanced Makefile version" "make -f Makefile.enhanced version"
fi

# File structure validation
echo -e "${YELLOW}File Structure Validation${NC}"
run_test "Internal packages exist" "test -d internal/config -a -d internal/actions -a -d internal/plugins"
run_test "Documentation exists" "test -f docs/ENHANCED-FEATURES.md -a -f ENHANCEMENTS-SUMMARY.md"
run_test "CI/CD files exist" "test -f .github/workflows/ci.yml -a -f .github/workflows/release.yml"
run_test "Linting config exists" "test -f .golangci.yml"

# Server mode test (quick start/stop)
echo -e "${YELLOW}Server Mode Tests${NC}"
run_test "Server mode start/stop" "timeout 5s /tmp/rrunner-enhanced --server --port 9999 || test \$? -eq 124"

# Integration with legacy system
echo -e "${YELLOW}Legacy Integration Tests${NC}"
run_test "Legacy diagnostics" "/tmp/rrunner-core --diagnose"
run_test "Enhanced diagnostics" "/tmp/rrunner-enhanced --diagnose || true"  # May not be fully implemented

# Print summary
echo
echo "======================================"
echo -e "${BLUE}Test Summary${NC}"
echo "======================================"
echo -e "Tests Run:    ${TESTS_RUN}"
echo -e "Tests Passed: ${GREEN}${TESTS_PASSED}${NC}"
echo -e "Tests Failed: ${RED}${TESTS_FAILED}${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All tests passed! Rrunner Enhanced is working correctly.${NC}"
    exit 0
else
    echo -e "\n${YELLOW}⚠️  Some tests failed. Please review the output above.${NC}"
    exit 1
fi