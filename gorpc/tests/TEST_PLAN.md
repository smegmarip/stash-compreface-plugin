# Comprehensive Test Plan for Stash Compreface Plugin
**Version:** 2.0.0 (Post-Refactor)
**Date:** 2025-11-09
**Status:** Planning Phase

---

## Executive Summary

This document outlines a comprehensive testing strategy for the refactored Stash Compreface plugin. The refactoring transformed a monolithic `package main` structure into properly organized Go packages with clear separation of concerns across ~5,000 lines of code.

### Refactored Architecture

```
gorpc/
├── main.go (minimal entry point)
├── internal/
│   ├── rpc/          # Business Logic Layer (869 lines)
│   ├── compreface/   # Compreface Client (573 lines)
│   ├── stash/        # Stash Domain (709 lines)
│   ├── vision/       # Vision Service Client (412 lines)
│   ├── quality/      # Quality Assessment (1,416 lines)
│   └── config/       # Configuration (226 lines)
└── pkg/utils/        # Shared Utilities
```

### Test Coverage Goals

1. **Unit Tests**: >80% coverage for all packages
2. **Integration Tests**: All external service interactions
3. **End-to-End Tests**: Complete workflows from task initiation to completion
4. **Performance Tests**: Batching, cooldown, memory stability
5. **Error Scenarios**: All failure modes handled gracefully

---

## Table of Contents

1. [Test Harness Structure](#1-test-harness-structure)
2. [Unit Testing Strategy](#2-unit-testing-strategy)
3. [Integration Testing Strategy](#3-integration-testing-strategy)
4. [End-to-End Testing Strategy](#4-end-to-end-testing-strategy)
5. [Test Data & Fixtures](#5-test-data--fixtures)
6. [Testing Tools & Infrastructure](#6-testing-tools--infrastructure)
7. [Implementation Timeline](#7-implementation-timeline)

---

## 1. Test Harness Structure

### 1.1 Directory Organization

```
gorpc/tests/
├── README.md                        # Testing documentation
├── fixtures/                        # Test data and fixtures
│   ├── images/                      # Test images (faces)
│   ├── performers/                  # Test performer data
│   └── responses/                   # Mock API responses
├── mocks/                           # Mock implementations
│   ├── compreface_mock.go          # Mock Compreface client
│   ├── stash_mock.go               # Mock GraphQL client
│   ├── vision_mock.go              # Mock Vision service
│   └── quality_mock.go             # Mock Quality service
├── testutil/                        # Shared test utilities
│   ├── assertions.go               # Custom assertions
│   ├── helpers.go                  # Helper functions
│   └── fixtures.go                 # Fixture loading
├── unit/                            # Unit tests by package
│   ├── config/
│   │   └── config_test.go
│   ├── compreface/
│   │   ├── client_test.go
│   │   ├── subjects_test.go
│   │   └── types_test.go
│   ├── stash/
│   │   ├── images_test.go
│   │   ├── performers_test.go
│   │   ├── scenes_test.go
│   │   ├── tags_test.go
│   │   └── cache_test.go
│   ├── vision/
│   │   └── client_test.go
│   ├── quality/
│   │   ├── detector_test.go
│   │   ├── filter_test.go
│   │   ├── fuzzy_test.go
│   │   ├── router_test.go
│   │   └── python_client_test.go
│   ├── rpc/
│   │   ├── service_test.go
│   │   ├── handlers_test.go
│   │   ├── performers_test.go
│   │   ├── images_test.go
│   │   ├── recognition_test.go
│   │   └── scenes_test.go
│   └── utils/
│       └── utils_test.go
├── integration/                     # Integration tests
│   ├── compreface/
│   │   ├── detection_test.go       # Live Compreface detection API
│   │   ├── recognition_test.go     # Live Compreface recognition API
│   │   └── subjects_test.go        # Live Compreface subjects API
│   ├── stash/
│   │   ├── graphql_test.go         # Live Stash GraphQL operations
│   │   └── metadata_test.go        # Live metadata operations
│   ├── vision/
│   │   └── vision_test.go          # Live Vision service API
│   └── quality/
│       └── quality_test.go         # Live Quality service API
├── e2e/                             # End-to-end workflow tests
│   ├── performer_sync_test.go
│   ├── image_recognition_test.go
│   ├── image_identification_test.go
│   ├── scene_recognition_test.go
│   └── full_workflow_test.go
├── performance/                     # Performance and load tests
│   ├── batching_test.go
│   ├── cooldown_test.go
│   ├── memory_test.go
│   └── concurrency_test.go
└── scenarios/                       # Error scenario tests
    ├── network_failures_test.go
    ├── api_errors_test.go
    ├── invalid_data_test.go
    └── cancellation_test.go
```

### 1.2 Organizational Principles

1. **Package Alignment**: Test structure mirrors source structure
2. **Test Types Separation**: Unit, integration, e2e, performance in separate directories
3. **Shared Resources**: Mocks, fixtures, and utilities centralized
4. **Self-Contained**: Each test file can run independently
5. **No Overlapping**: Clear boundaries between test types

---

## 2. Unit Testing Strategy

### 2.1 Package: config

**File**: `tests/unit/config/config_test.go`

**Coverage Target**: 90%

**Test Cases**:
```go
// Configuration Loading
TestLoad                           // Load from plugin input
TestLoadDefaults                   // Default values applied
TestLoadWithPartialConfig          // Partial config + defaults
TestLoadValidation                 // Invalid config rejected

// Service URL Resolution
TestResolveServiceURL_Localhost    // localhost preserved
TestResolveServiceURL_IP           // IP addresses used as-is
TestResolveServiceURL_Hostname     // DNS resolution
TestResolveServiceURL_Container    // Container name resolution
TestResolveServiceURL_InvalidDNS   // DNS failure fallback
```

**Mock Dependencies**: Stash plugin input, DNS resolver

---

### 2.2 Package: compreface

**File**: `tests/unit/compreface/client_test.go`

**Coverage Target**: 85%

**Test Cases**:
```go
// Client Initialization
TestNewClient                      // Client creation
TestClientWithTimeout              // Custom timeout

// Face Detection
TestDetectFaces_Success            // Successful detection
TestDetectFaces_NoFaces            // Image with no faces
TestDetectFaces_MultipleFaces      // Multiple faces detected
TestDetectFaces_APIError           // API error handling
TestDetectFaces_InvalidImage       // Invalid image file

// Face Recognition
TestRecognizeFaces_Match           // Face matches existing subject
TestRecognizeFaces_NoMatch         // Face doesn't match
TestRecognizeFaces_MultipleFaces   // Multiple faces recognized
TestRecognizeFaces_LowSimilarity   // Below threshold

// Subject Management
TestAddSubject_Success             // Create new subject
TestAddSubject_Duplicate           // Duplicate subject handling
TestListSubjects                   // List all subjects
TestDeleteSubject                  // Delete subject
TestListFaces                      // List subject faces
TestDeleteFace                     // Delete face image

// Error Handling
TestHandleHTTPError_400            // Bad request
TestHandleHTTPError_401            // Unauthorized
TestHandleHTTPError_404            // Not found
TestHandleHTTPError_500            // Server error
TestHandleHTTPError_Timeout        // Request timeout
```

**File**: `tests/unit/compreface/subjects_test.go`

**Test Cases**:
```go
// Subject Naming
TestRandomSubject_Length           // Correct length
TestRandomSubject_Charset          // Only A-Z, 0-9
TestRandomSubject_Prefix           // Prefix preserved
TestRandomSubject_Randomness       // Different calls produce different results

TestCreateSubjectName_Format       // "Person {id} {random}" format
TestCreateSubjectName_Length       // Total length correct
TestCreateSubjectName_Uniqueness   // Unique for same ID

// Alias Matching
TestFindPersonAlias_InAliases      // Found in alias list
TestFindPersonAlias_InName         // Found in performer name
TestFindPersonAlias_NotFound       // Not found
TestFindPersonAlias_EmptyAliases   // No aliases
TestPersonAliasPattern             // Regex pattern matching
```

---

### 2.3 Package: stash

**File**: `tests/unit/stash/tags_test.go`

**Test Cases**:
```go
// Tag Operations
TestGetOrCreateTag_ExistingTag     // Tag exists
TestGetOrCreateTag_CreateNew       // Tag doesn't exist
TestGetOrCreateTag_CacheHit        // Tag in cache
TestGetOrCreateTag_GraphQLError    // GraphQL error

TestTagCache_Get                   // Cache retrieval
TestTagCache_Set                   // Cache storage
TestTagCache_Concurrent            // Thread safety
```

**File**: `tests/unit/stash/images_test.go`

**Test Cases**:
```go
// Image Repository
TestFindImages_WithFilter          // With tag filter
TestFindImages_Pagination          // Pagination works
TestFindImages_Empty               // No results
TestGetImage_Success               // Image found
TestGetImage_NotFound              // Image not found

// Image Updates
TestUpdateImage_Tags               // Update image tags
TestUpdateImage_Performers         // Update image performers
TestAddTagToImage                  // Add tag
TestRemoveTagFromImage             // Remove tag
```

**File**: `tests/unit/stash/performers_test.go`

**Test Cases**:
```go
// Performer Repository
TestFindPerformers                 // Find performers
TestFindPerformerByAlias           // Find by alias
TestGetPerformer                   // Get by ID
TestCreatePerformer                // Create new
TestUpdatePerformer                // Update existing
```

**File**: `tests/unit/stash/scenes_test.go`

**Test Cases**:
```go
// Scene Repository
TestFindScenes                     // Find scenes
TestGetScene                       // Get by ID
TestUpdateSceneTags                // Update tags
TestUpdateScenePerformers          // Update performers
```

---

### 2.4 Package: vision

**File**: `tests/unit/vision/client_test.go`

**Test Cases**:
```go
// Vision Service Client
TestNewVisionClient                // Client creation
TestHealthCheck_Available          // Service available
TestHealthCheck_Unavailable        // Service down
TestIsAvailable                    // Availability check

// Job Management
TestSubmitJob                      // Job submission
TestGetJobStatus                   // Status polling
TestWaitForCompletion_Success      // Job completes
TestWaitForCompletion_Failed       // Job fails
TestWaitForCompletion_Timeout      // Job timeout

// Face Recognition
TestBuildRecognizeFacesRequest     // Request builder
TestGetResults                     // Result retrieval
```

---

### 2.5 Package: quality

**File**: `tests/unit/quality/detector_test.go`

**Test Cases**:
```go
// Go-face Detector
TestDetectFaces_Single             // Single face
TestDetectFaces_Multiple           // Multiple faces
TestDetectFaces_None               // No faces
TestDetectFaces_Landmarks          // Landmark detection
TestQualityScore                   // Quality scoring
```

**File**: `tests/unit/quality/filter_test.go`

**Test Cases**:
```go
// Quality Filtering
TestFilterByConfidence             // Confidence threshold
TestFilterBySize                   // Minimum size
TestFilterByPose                   // Pose filtering
TestApplyQualityFilter             // Combined filters
```

**File**: `tests/unit/quality/router_test.go`

**Test Cases**:
```go
// Quality Router
TestRoute_GoInternal               // Route to Go detector
TestRoute_PythonService            // Route to Python service
TestRoute_Auto                     // Auto selection
TestRoute_Fallback                 // Fallback mechanism
```

---

### 2.6 Package: rpc

**File**: `tests/unit/rpc/service_test.go`

**Test Cases**:
```go
// Service Lifecycle
TestNewService                     // Service creation
TestStop                           // Graceful shutdown
TestApplyCooldown                  // Cooldown application

// Configuration
TestRun_LoadConfig                 // Config loading
TestRun_InitClients                // Client initialization
TestErrorOutput                    // Error output formatting
```

**File**: `tests/unit/rpc/handlers_test.go`

**Test Cases**:
```go
// Task Routing
TestRouteTask_SynchronizePerformers
TestRouteTask_RecognizeImagesHQ
TestRouteTask_RecognizeImagesLQ
TestRouteTask_IdentifyImagesAll
TestRouteTask_IdentifyImagesNew
TestRouteTask_ResetUnmatched
TestRouteTask_RecognizeScenes
TestRouteTask_IdentifyImage
TestRouteTask_CreatePerformer
TestRouteTask_IdentifyGallery
TestRouteTask_UnknownMode          // Unknown mode error
```

**File**: `tests/unit/rpc/performers_test.go`

**Test Cases**:
```go
// Performer Synchronization
TestSynchronizePerformers_Success
TestSynchronizePerformers_EmptyList
TestSynchronizePerformers_WithErrors
TestSyncPerformer_NewSubject
TestSyncPerformer_ExistingSubject
TestSyncPerformer_NoImage
TestSyncPerformer_NoAlias
```

**File**: `tests/unit/rpc/images_test.go`

**Test Cases**:
```go
// Image Recognition
TestIdentifyImage_CreatePerformer
TestIdentifyImage_MatchExisting
TestIdentifyImage_NoFaces
TestIdentifyImage_MultipleFaces

// Image Identification
TestIdentifyImages_All
TestIdentifyImages_New
TestIdentifyImages_WithFilter

// Gallery Processing
TestIdentifyGallery
TestResetUnmatchedImages
```

**File**: `tests/unit/rpc/recognition_test.go`

**Test Cases**:
```go
// Recognition Workflow
TestRecognizeImages_HQ
TestRecognizeImages_LQ
TestRecognizeImageFaces
TestHandleFaceDetection
TestHandleFaceRecognition
TestCreatePerformerFromFace
```

**File**: `tests/unit/rpc/scenes_test.go`

**Test Cases**:
```go
// Scene Recognition
TestRecognizeScenes_WithVision
TestRecognizeScenes_NoVision
TestRecognizeScenes_Sprites
TestProcessSceneFaces
TestMatchFaceToSubject
```

---

### 2.7 Package: utils

**File**: `tests/unit/utils/utils_test.go`

**Test Cases**:
```go
// Utility Functions
TestRandomSubject                  // Subject name generation
TestDeduplicateIDs                 // ID deduplication
TestMergePerformerIDs              // Performer ID merging
TestCropFaceFromImage              // Face cropping
TestResolveServiceURL              // URL resolution
TestMin                            // Min function
TestJoinStrings                    // String joining
```

---

## 3. Integration Testing Strategy

### 3.1 Compreface Integration

**File**: `tests/integration/compreface/detection_test.go`

**Prerequisites**: Running Compreface instance at localhost:8000

**Test Cases**:
```go
TestLiveDetection_SingleFace
TestLiveDetection_MultipleFaces
TestLiveDetection_NoFaces
TestLiveDetection_LargeImage
TestLiveDetection_SmallImage
TestLiveDetection_InvalidImage
TestLiveDetection_Concurrency      // Multiple concurrent requests
```

**File**: `tests/integration/compreface/recognition_test.go`

**Test Cases**:
```go
TestLiveRecognition_Match
TestLiveRecognition_NoMatch
TestLiveRecognition_MultipleSubjects
TestLiveRecognition_SimilarityThreshold
```

**File**: `tests/integration/compreface/subjects_test.go`

**Test Cases**:
```go
TestLiveSubject_CRUD               // Create, Read, Update, Delete
TestLiveSubject_ListAll
TestLiveSubject_DuplicateName
TestLiveSubject_FaceManagement
```

---

### 3.2 Stash Integration

**File**: `tests/integration/stash/graphql_test.go`

**Prerequisites**: Running Stash instance at localhost:9999

**Test Cases**:
```go
TestLiveGraphQL_TagOperations
TestLiveGraphQL_ImageOperations
TestLiveGraphQL_PerformerOperations
TestLiveGraphQL_SceneOperations
TestLiveGraphQL_Mutations
TestLiveGraphQL_MetadataScan
```

---

### 3.3 Vision Service Integration

**File**: `tests/integration/vision/vision_test.go`

**Prerequisites**: Running Vision Service at localhost:5000

**Test Cases**:
```go
TestLiveVision_HealthCheck
TestLiveVision_JobSubmission
TestLiveVision_JobPolling
TestLiveVision_ResultRetrieval
TestLiveVision_FaceDetection
TestLiveVision_FrameExtraction
```

---

### 3.4 Quality Service Integration

**File**: `tests/integration/quality/quality_test.go`

**Prerequisites**: Running Quality Service at localhost:6001

**Test Cases**:
```go
TestLiveQuality_Assess
TestLiveQuality_Preprocess
TestLiveQuality_Detect
TestLiveQuality_MultipleImages
```

---

## 4. End-to-End Testing Strategy

### 4.1 Performer Synchronization Workflow

**File**: `tests/e2e/performer_sync_test.go`

**Scenario**: Synchronize existing Stash performers with Compreface

**Steps**:
1. Create test performers with images and aliases
2. Execute "Synchronize Performers" task
3. Verify Compreface subjects created
4. Verify subject names match aliases
5. Verify no duplicates
6. Clean up test data

**Validation**:
- All performers processed
- Compreface subjects created correctly
- Subject naming format preserved
- Progress reporting accurate

---

### 4.2 Image Recognition Workflow

**File**: `tests/e2e/image_recognition_test.go`

**Scenario**: Detect and group faces in unprocessed images

**Steps**:
1. Upload test images (no existing tags)
2. Execute "Recognize Images (HQ)" task
3. Verify faces detected
4. Verify performers created
5. Verify tags applied
6. Clean up test data

**Validation**:
- All faces detected
- New performers created for unknown faces
- Images tagged "Compreface Scanned"
- Progress updates visible

---

### 4.3 Image Identification Workflow

**File**: `tests/e2e/image_identification_test.go`

**Scenario**: Match faces in images to existing performers

**Steps**:
1. Have existing performers with Compreface subjects
2. Upload new images of same people
3. Execute "Identify Unscanned Images" task
4. Verify matches to existing performers
5. Verify no duplicate performers
6. Clean up test data

**Validation**:
- Faces matched correctly
- Similarity scores above threshold
- Images updated with performers
- Matched tags applied

---

### 4.4 Scene Recognition Workflow

**File**: `tests/e2e/scene_recognition_test.go`

**Prerequisites**: Vision Service running

**Scenario**: Extract and recognize faces from video scenes

**Steps**:
1. Upload test video to Stash
2. Execute "Recognize Scenes" task
3. Verify Vision Service job submitted
4. Verify faces detected in video
5. Verify performers matched/created
6. Verify scene tagged
7. Clean up test data

**Validation**:
- Vision Service integration works
- Faces extracted from video
- Face-to-performer matching correct
- Scene metadata updated

---

### 4.5 Full Workflow

**File**: `tests/e2e/full_workflow_test.go`

**Scenario**: Complete workflow from setup to final state

**Steps**:
1. Fresh Stash + Compreface instances
2. Create sample performers
3. Sync performers
4. Recognize all images
5. Identify matches
6. Process galleries
7. Reset unmatched
8. Verify final state

**Validation**:
- Complete workflow success
- No errors
- All data consistent
- Performance acceptable

---

## 5. Test Data & Fixtures

### 5.1 Test Images

**Location**: `tests/fixtures/images/`

**Categories**:
```
tests/fixtures/images/
├── single_face/           # Clear single face (10 images)
├── multiple_faces/        # 2-5 faces (10 images)
├── no_face/              # No faces (5 images)
├── low_quality/          # Blurry, small (10 images)
├── high_quality/         # Clear, large (10 images)
├── edge_cases/           # Side profiles, obscured (10 images)
└── invalid/              # Corrupted files (5 files)
```

**Total**: ~60 test images + 5 invalid files

### 5.2 Mock API Responses

**Location**: `tests/fixtures/responses/`

**Files**:
```
tests/fixtures/responses/
├── compreface/
│   ├── detection_success.json
│   ├── detection_no_faces.json
│   ├── recognition_match.json
│   ├── recognition_no_match.json
│   ├── subject_created.json
│   └── error_responses.json
├── stash/
│   ├── find_images.json
│   ├── find_performers.json
│   ├── tag_created.json
│   └── performer_created.json
└── vision/
    ├── job_submitted.json
    ├── job_status.json
    ├── job_results.json
    └── health_check.json
```

### 5.3 Test Performers

**Location**: `tests/fixtures/performers/`

**Data**:
```json
{
  "performers": [
    {
      "name": "Test Performer 1",
      "aliases": ["Person 1001 ABCD1234EFGH5678"],
      "image": "fixtures/images/single_face/performer1.jpg"
    },
    {
      "name": "Test Performer 2",
      "aliases": ["Person 1002 IJKL5678MNOP9012"],
      "image": "fixtures/images/single_face/performer2.jpg"
    }
  ]
}
```

---

## 6. Testing Tools & Infrastructure

### 6.1 Testing Libraries

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)
```

### 6.2 Mock Generation

Use `mockery` for interface mocks:
```bash
mockery --name=Client --dir=internal/compreface --output=tests/mocks
mockery --name=GraphQLClient --dir=internal/stash --output=tests/mocks
```

### 6.3 Test Helpers

**File**: `tests/testutil/helpers.go`

```go
package testutil

// SetupTestEnv prepares test environment
func SetupTestEnv(t *testing.T) (*TestEnv, func())

// LoadFixture loads test fixture
func LoadFixture(path string) []byte

// CreateTestImage creates test image with face
func CreateTestImage(width, height int, numFaces int) []byte

// AssertSubjectNameFormat validates subject name format
func AssertSubjectNameFormat(t *testing.T, name string, imageID string)
```

### 6.4 Docker Compose for Testing

**File**: `tests/docker-compose.yml`

```yaml
version: '3.8'
services:
  stash-test:
    image: stashapp/stash:latest
    ports:
      - "19999:9999"
    volumes:
      - ./test-data:/data

  compreface-test:
    image: exadel/compreface:latest
    ports:
      - "18000:8000"
    environment:
      - API_JAVA_OPTS=-Xmx8g

  vision-test:
    image: stash-auto-vision:latest
    ports:
      - "15000:5000"

  quality-test:
    image: stash-face-quality:latest
    ports:
      - "16001:6001"
```

### 6.5 CI/CD Integration

**File**: `.github/workflows/test.yml`

```yaml
name: Test Suite
on: [push, pull_request]
jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Unit Tests
        run: |
          cd gorpc
          go test ./tests/unit/... -v -cover

  integration:
    runs-on: ubuntu-latest
    services:
      compreface:
        image: exadel/compreface:latest
        ports:
          - 8000:8000
    steps:
      - uses: actions/checkout@v3
      - name: Integration Tests
        run: |
          cd gorpc
          go test ./tests/integration/... -v

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Services
        run: docker-compose -f tests/docker-compose.yml up -d
      - name: E2E Tests
        run: |
          cd gorpc
          go test ./tests/e2e/... -v
```

---

## 7. Implementation Timeline

### Week 1: Foundation (Days 1-5)
- Day 1: Create test harness structure
- Day 2-3: Implement mocks and fixtures
- Day 4-5: Test utilities and helpers

### Week 2: Unit Tests (Days 6-10)
- Day 6: config, utils packages
- Day 7: compreface package
- Day 8: stash package
- Day 9: vision, quality packages
- Day 10: rpc package

### Week 3: Integration Tests (Days 11-15)
- Day 11-12: Compreface integration tests
- Day 13: Stash integration tests
- Day 14: Vision & Quality integration tests
- Day 15: Integration test refinement

### Week 4: E2E & Performance (Days 16-20)
- Day 16-17: E2E workflow tests
- Day 18: Performance tests
- Day 19: Error scenario tests
- Day 20: Final validation and documentation

---

## Success Criteria

1. ✅ **Coverage**: >80% unit test coverage
2. ✅ **Integration**: All external services tested
3. ✅ **E2E**: All 11 tasks have workflow tests
4. ✅ **Performance**: Batch processing verified
5. ✅ **Errors**: All failure modes tested
6. ✅ **CI/CD**: Automated test execution
7. ✅ **Documentation**: All tests documented

---

## Next Steps

1. **Review & Approve**: Review this plan with stakeholders
2. **Setup Infrastructure**: Create directory structure
3. **Generate Mocks**: Create mock implementations
4. **Implement Unit Tests**: Start with smallest packages
5. **Integration Tests**: Test against live services
6. **E2E Tests**: Complete workflow validation
7. **Performance Tests**: Validate batching and cooldown
8. **CI/CD**: Automate test execution

---

**Approval Required**: Yes
**Estimated Effort**: 4 weeks (80 hours)
**Risk Level**: Medium (dependency on live services)
