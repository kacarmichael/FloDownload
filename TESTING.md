# Testing Guide

This document describes the test suite for the StreamRecorder application.

## Overview

The test suite provides comprehensive coverage of core application components without requiring external dependencies like video files, NAS connectivity, or FFmpeg. All tests are self-contained and clean up after themselves.

## Test Structure

### Unit Tests by Package

#### `pkg/config` 
- **File**: `config_test.go`
- **Coverage**: Configuration loading, environment variable override, path validation, validation errors
- **Key Tests**:
  - Default config loading
  - Environment variable overrides
  - Path resolution and creation
  - Validation error scenarios

#### `pkg/utils`
- **File**: `paths_test.go` 
- **Coverage**: Cross-platform path utilities, directory operations, validation
- **Key Tests**:
  - Safe path joining
  - Directory creation
  - Path existence checking
  - Path validation
  - Write permission testing

#### `pkg/constants`
- **File**: `constants_test.go`
- **Coverage**: Constants values, configuration singleton, integration
- **Key Tests**:
  - Constant value verification
  - Singleton pattern testing
  - Config integration
  - Concurrent access safety

#### `pkg/httpClient`
- **File**: `error_test.go`
- **Coverage**: HTTP error handling, status code management
- **Key Tests**:
  - HTTP error creation and formatting
  - Error comparison and detection
  - Status code extraction
  - Error wrapping support

#### `pkg/media`
- **File**: `manifest_test.go`
- **Coverage**: Manifest generation, segment tracking, JSON serialization
- **Key Tests**:
  - Manifest writer initialization
  - Segment addition and updates
  - Quality resolution logic
  - JSON file generation
  - Sorting and deduplication

#### `pkg/processing`
- **File**: `service_test.go`
- **Coverage**: Processing service logic, path resolution, FFmpeg handling
- **Key Tests**:
  - Service initialization
  - Event directory scanning
  - Resolution detection
  - Segment aggregation
  - File concatenation list generation
  - FFmpeg path resolution

## Running Tests

### Quick Test Run
```bash
make test
```

### Verbose Output
```bash
make test-verbose
```

### Coverage Report
```bash
make test-coverage
```
Generates `coverage.html` with detailed coverage report.

### Test Specific Package
```bash
make test-pkg PKG=./pkg/config
```

### Manual Test Execution
```bash
# Run custom test runner
go run test_runner.go

# Run standard go test
go test ./pkg/...

# Run with coverage
go test -coverprofile=coverage.out ./pkg/...
go tool cover -html=coverage.out
```

## Test Features

### ✅ Self-Contained
- No external file dependencies
- No network connections required
- No NAS or FFmpeg installation needed

### ✅ Automatic Cleanup
- All temporary files/directories removed after tests
- Original environment variables restored
- No side effects on host system

### ✅ Isolated Environment
- Tests use temporary directories
- Environment variables safely overridden
- Configuration isolated from production settings

### ✅ Cross-Platform
- Path handling tested on Windows/Unix
- Platform-specific behavior validated
- Cross-platform compatibility verified

### ✅ Comprehensive Coverage
- Configuration management
- Path utilities and validation
- Error handling patterns
- Data structures and serialization
- Business logic without external dependencies

## Test Environment

The test suite automatically:

1. **Creates Temporary Workspace**: Each test run uses a fresh temporary directory
2. **Sets Test Environment**: Overrides environment variables to use test settings
3. **Disables External Dependencies**: Sets flags to disable NAS transfer and processing
4. **Cleans Up Completely**: Removes all test artifacts and restores environment

### Environment Variables Set During Tests
- `LOCAL_OUTPUT_DIR`: Points to temp directory
- `PROCESS_OUTPUT_DIR`: Points to temp directory  
- `ENABLE_NAS_TRANSFER`: Set to `false`
- `PROCESSING_ENABLED`: Set to `false`

## Extending Tests

### Adding New Test Cases

1. **Create test file**: `pkg/yourpackage/yourfile_test.go`
2. **Follow naming convention**: `TestFunctionName` 
3. **Use temp directories**: Always clean up created files
4. **Mock external dependencies**: Avoid real file operations where possible

### Test Template
```go
package yourpackage

import (
    "os"
    "testing"
)

func TestYourFunction(t *testing.T) {
    // Setup
    tempDir, err := os.MkdirTemp("", "test_*")
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    // Test logic
    result := YourFunction()
    
    // Assertions
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Best Practices

- **Always clean up**: Use `defer os.RemoveAll()` for temp directories
- **Test error cases**: Don't just test happy paths
- **Use table-driven tests**: For multiple similar test cases
- **Mock external dependencies**: Use echo/dummy commands instead of real tools
- **Validate cleanup**: Ensure tests don't leave artifacts

## CI/CD Integration

The test suite is designed for automated environments:

```bash
# Complete CI pipeline
make ci

# Just run tests in CI
make test
```

The custom test runner provides:
- ✅ Colored output for easy reading
- ✅ Test count and timing statistics  
- ✅ Failure details and summaries
- ✅ Automatic environment management
- ✅ Exit codes for CI integration

## Troubleshooting

### Common Issues

**Tests fail with permission errors**
- Ensure write permissions in temp directory
- Check antivirus software isn't blocking file operations

**Config tests fail**  
- Verify no conflicting environment variables are set
- Check that temp directories can be created

**Path tests fail on Windows**
- Confirm path separator handling is correct
- Verify Windows path validation logic

### Debug Mode
```bash
# Run with verbose output to see detailed failures
go test -v ./pkg/...

# Run specific failing test
go test -v -run TestSpecificFunction ./pkg/config
```

## Coverage Goals

Current test coverage targets:
- **Configuration**: 95%+ (critical for startup validation)
- **Path utilities**: 90%+ (cross-platform compatibility critical)
- **Constants**: 85%+ (verify all values and singleton behavior)
- **HTTP client**: 90%+ (error handling is critical)
- **Media handling**: 85%+ (core business logic)
- **Processing**: 70%+ (limited by external FFmpeg dependency)

Generate coverage report to verify:
```bash
make test-coverage
open coverage.html
```