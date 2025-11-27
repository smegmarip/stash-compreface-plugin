# Stash Compreface Plugin - Test Harness

**Status**: Planning Complete - Ready for Implementation
**Date**: 2025-11-09
**Version**: 2.0.0 (Post-Refactor)

---

## Overview

This testing harness provides comprehensive test coverage for the refactored Stash Compreface plugin. The refactoring transformed ~5,000 lines of monolithic code into properly organized Go packages with clear separation of concerns.

## Quick Start

### Run All Tests

```bash
cd gorpc
export TMPDIR=/tmp GOTMPDIR=/tmp
go test ./tests/... -v
```

### Run Specific Test Suites

```bash
# Unit tests only
go test ./tests/unit/... -v

# Integration tests (requires services)
go test ./tests/integration/... -v -tags=integration

# E2E tests (requires full environment)
go test ./tests/e2e/... -v -tags=e2e

# Performance tests
go test ./tests/performance/... -v -bench=.
```

### Run with Coverage

```bash
go test ./tests/unit/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Test Structure

```
tests/
├── README.md                 # This file
├── TEST_PLAN.md              # Comprehensive testing plan
├── fixtures/                 # Test data
├── mocks/                    # Mock implementations
├── testutil/                 # Shared utilities
├── unit/                     # Unit tests (no external deps)
├── integration/              # Integration tests (live services)
├── e2e/                      # End-to-end workflow tests
├── performance/              # Performance & load tests
└── scenarios/                # Error scenario tests
```

---

## Test Coverage by Package

### Core Packages (Target: >80%)

| Package             | Lines | Unit Tests  | Integration   | E2E           | Priority |
| ------------------- | ----- | ----------- | ------------- | ------------- | -------- |
| internal/rpc        | ~900  | ✅ Required | ✅ Required   | ✅ Required   | **HIGH** |
| internal/compreface | ~600  | ✅ Required | ✅ Required   | ⚠️ Optional   | **HIGH** |
| internal/stash      | ~700  | ✅ Required | ✅ Required   | ⚠️ Optional   | **HIGH** |
| internal/vision     | ~450  | ✅ Required | ✅ Required   | ⚠️ Optional   | MEDIUM   |
| internal/config     | ~230  | ✅ Required | ⚠️ Optional   | ⚠️ Optional   | HIGH     |
| pkg/utils           | ~200  | ✅ Required | ❌ Not Needed | ❌ Not Needed | MEDIUM   |

**Legend**:

- ✅ Required - Must implement
- ⚠️ Optional - Nice to have
- ❌ Not Needed - Not applicable

---

## Test Types

### 1. Unit Tests (`tests/unit/`)

**Purpose**: Test individual functions and methods in isolation

**Characteristics**:

- No external dependencies
- Uses mocks for all I/O
- Fast execution (<1s per package)
- Can run without services

**Example**:

```go
func TestCreateSubjectName(t *testing.T) {
    name := compreface.CreateSubjectName("12345")

    assert.True(t, strings.HasPrefix(name, "Person 12345 "))
    assert.Equal(t, len(name), len("Person 12345 ")+16)

    // Verify only A-Z and 0-9 in random part
    randomPart := name[len("Person 12345 "):]
    for _, c := range randomPart {
        assert.True(t, (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'))
    }
}
```

**Coverage Goal**: >80% for all packages

---

### 2. Integration Tests (`tests/integration/`)

**Purpose**: Test interactions with external services

**Requirements**:

- Running Compreface (localhost:8000)
- Running Stash (localhost:9999)
- Running Vision Service (localhost:5010) - optional

**Example**:

```go
// +build integration

func TestLiveDetection_SingleFace(t *testing.T) {
    client := compreface.NewClient(
        "http://localhost:8000",
        os.Getenv("COMPREFACE_RECOGNITION_KEY"),
        os.Getenv("COMPREFACE_DETECTION_KEY"),
        "",
        0.81,
    )

    result, err := client.DetectFaces("fixtures/images/single_face/test1.jpg")

    require.NoError(t, err)
    assert.Len(t, result.Result, 1)
    assert.Greater(t, result.Result[0].Confidence, 0.9)
}
```

**Run with**: `go test -tags=integration ./tests/integration/...`

---

### 3. End-to-End Tests (`tests/e2e/`)

**Purpose**: Test complete workflows from start to finish

**Requirements**:

- All services running
- Test data prepared
- Clean initial state

**Example**:

```go
// +build e2e

func TestPerformerSyncWorkflow(t *testing.T) {
    // Setup
    env := testutil.SetupE2EEnv(t)
    defer env.Teardown()

    // Create test performers
    performers := env.CreateTestPerformers(5)

    // Execute sync
    service := rpc.NewService()
    output, err := service.SynchronizePerformers()

    // Verify
    require.NoError(t, err)
    assert.Contains(t, output, "5 performers")

    // Check Compreface
    subjects := env.ListComprefaceSubjects()
    assert.Len(t, subjects, 5)
}
```

**Run with**: `go test -tags=e2e ./tests/e2e/...`

---

### 4. Performance Tests (`tests/performance/`)

**Purpose**: Validate performance characteristics

**Metrics**:

- Batch processing speed
- Memory stability
- Cooldown effectiveness
- Concurrent request handling

**Example**:

```go
func BenchmarkBatchProcessing(b *testing.B) {
    service := setupTestService()
    images := generateTestImages(20)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        service.RecognizeImages(images, false)
    }
}

func TestMemoryStability(t *testing.T) {
    // Process 1000 images and verify no memory leak
    initialMem := runtime.MemStats{}
    runtime.ReadMemStats(&initialMem)

    // Process images...

    finalMem := runtime.MemStats{}
    runtime.ReadMemStats(&finalMem)

    growth := finalMem.Alloc - initialMem.Alloc
    assert.Less(t, growth, 100*1024*1024) // <100MB growth
}
```

---

### 5. Error Scenario Tests (`tests/scenarios/`)

**Purpose**: Verify error handling and recovery

**Scenarios**:

- Network failures
- API errors (4xx, 5xx)
- Invalid data
- Task cancellation
- Service unavailability

**Example**:

```go
func TestComprefaceUnavailable(t *testing.T) {
    // Use invalid URL
    client := compreface.NewClient("http://localhost:99999", "", "", "", 0.81)

    _, err := client.DetectFaces("test.jpg")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "connection refused")
}
```

---

## Test Utilities

### Mock Implementations (`tests/mocks/`)

Generated mocks for all interfaces:

```go
// CompreFaceClient mock
type MockComprefaceClient struct {
    mock.Mock
}

func (m *MockComprefaceClient) DetectFaces(path string) (*compreface.DetectionResponse, error) {
    args := m.Called(path)
    return args.Get(0).(*compreface.DetectionResponse), args.Error(1)
}
```

### Test Helpers (`tests/testutil/`)

Shared utilities for all tests:

```go
// Setup test environment
func SetupTestEnv(t *testing.T) (*TestEnv, func())

// Load test fixtures
func LoadFixture(path string) []byte

// Assert subject name format
func AssertSubjectNameFormat(t *testing.T, name string, imageID string)

// Create test images
func CreateTestImage(width, height int, numFaces int) []byte
```

### Fixtures (`tests/fixtures/`)

Test data organized by type:

```
fixtures/
├── images/
│   ├── single_face/      # 10 images with 1 face
│   ├── multiple_faces/   # 10 images with 2-5 faces
│   ├── no_face/          # 5 images with no faces
│   ├── low_quality/      # 10 blurry/small images
│   └── high_quality/     # 10 clear/large images
├── responses/
│   ├── compreface/       # Mock API responses
│   ├── stash/            # Mock GraphQL responses
│   └── vision/           # Mock Vision responses
└── performers/
    └── test_data.json    # Test performer definitions
```

---

## Environment Setup

### Prerequisites

```bash
# Go version
go version  # Should be 1.21+

# Docker services (for integration tests)
docker-compose -f tests/docker-compose.yml up -d

# Environment variables
export STASH_URL="http://localhost:9999"
export COMPREFACE_URL="http://localhost:8000"
export COMPREFACE_RECOGNITION_KEY="your_key"
export COMPREFACE_DETECTION_KEY="your_key"
```

### Docker Test Environment

```bash
# Start all services
docker-compose -f tests/docker-compose.yml up -d

# Check service health
curl http://localhost:19999/graphql  # Stash
curl http://localhost:18000/         # Compreface
curl http://localhost:15000/health   # Vision

# Stop all services
docker-compose -f tests/docker-compose.yml down
```

---

## Test Execution

### Local Development

```bash
# Run unit tests (fast, no services needed)
go test ./tests/unit/... -v

# Run integration tests (requires services)
docker-compose -f tests/docker-compose.yml up -d
go test -tags=integration ./tests/integration/... -v

# Run specific package tests
go test ./tests/unit/compreface/... -v

# Run with race detection
go test -race ./tests/...

# Run with coverage
go test ./tests/unit/... -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

### CI/CD Pipeline

Tests run automatically on:

- Push to main branch
- Pull requests
- Manual workflow dispatch

See `.github/workflows/test.yml` for configuration.

---

## Test Data Management

### Creating Test Data

```bash
# Generate test images
go run tests/tools/generate_test_images.go

# Load test performers
go run tests/tools/load_test_performers.go

# Seed Compreface with subjects
go run tests/tools/seed_compreface.go
```

### Cleaning Test Data

```bash
# Remove all Compreface test subjects
curl -X DELETE http://localhost:8000/api/v1/recognition/subjects \
  -H "x-api-key: $COMPREFACE_RECOGNITION_KEY"

# Clean Stash test data
go run tests/tools/clean_stash_data.go

# Reset test environment
go run tests/tools/reset_test_env.go
```

---

## Debugging Tests

### Verbose Output

```bash
go test -v ./tests/unit/compreface/...
```

### Run Single Test

```bash
go test -v -run TestCreateSubjectName ./tests/unit/compreface/...
```

### Debug with Delve

```bash
dlv test ./tests/unit/compreface/... -- -test.run TestCreateSubjectName
```

### Enable Debug Logging

```bash
export COMPREFACE_LOG_LEVEL=debug
go test -v ./tests/...
```

---

## Contributing Tests

### Test Naming Conventions

```go
// Unit tests
func TestFunctionName_Scenario(t *testing.T)
func TestFunctionName_ErrorCase(t *testing.T)

// Table-driven tests
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    interface{}
        expected interface{}
    }{
        {"scenario 1", input1, expected1},
        {"scenario 2", input2, expected2},
    }
    // ...
}

// Integration tests
// +build integration
func TestLiveServiceName_Operation(t *testing.T)

// E2E tests
// +build e2e
func TestWorkflowName(t *testing.T)
```

### Test Structure

```go
func TestExample(t *testing.T) {
    // Setup
    setup := setupTestEnv(t)
    defer setup.Teardown()

    // Execute
    result, err := functionUnderTest(input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Adding New Tests

1. Identify which test type (unit/integration/e2e)
2. Create test file in appropriate directory
3. Use mocks for unit tests
4. Use real services for integration tests
5. Document any special requirements
6. Add to CI/CD pipeline if needed

---

## Troubleshooting

### Common Issues

**Issue**: Tests fail with "permission denied"

```bash
# Solution: Set temp directories
export TMPDIR=/tmp
export GOTMPDIR=/tmp
```

**Issue**: Integration tests can't connect to services

```bash
# Solution: Check services are running
docker-compose -f tests/docker-compose.yml ps
curl http://localhost:8000/  # Check Compreface
```

**Issue**: E2E tests leave dirty data

```bash
# Solution: Run cleanup script
go run tests/tools/reset_test_env.go
```

**Issue**: Tests timeout

```bash
# Solution: Increase timeout
go test -timeout 10m ./tests/...
```

---

## Resources

- **Full Test Plan**: See `TEST_PLAN.md` for comprehensive details
- **Testing Docs**: See `../../docs/TESTING.md` for methodology
- **CLAUDE.md**: See implementation phase documentation
- **CI/CD**: See `.github/workflows/test.yml`

---

## Status

| Category          | Status     | Coverage      | Tests      |
| ----------------- | ---------- | ------------- | ---------- |
| Unit Tests        | ⏳ Planned | Target: 80%   | ~150 tests |
| Integration Tests | ⏳ Planned | Target: 100%  | ~30 tests  |
| E2E Tests         | ⏳ Planned | All workflows | ~10 tests  |
| Performance Tests | ⏳ Planned | Key metrics   | ~5 tests   |
| Error Scenarios   | ⏳ Planned | All modes     | ~10 tests  |

**Legend**:

- ✅ Complete
- 🔄 In Progress
- ⏳ Planned
- ❌ Blocked

---

**Next Steps**:

1. Review and approve TEST_PLAN.md
2. Create directory structure
3. Generate mocks
4. Implement unit tests (priority: config, compreface, stash)
5. Implement integration tests
6. Implement E2E tests

**Estimated Effort**: 4 weeks (80 hours)
**Start Date**: TBD
**Target Completion**: TBD
