package mocks

import (
	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/stretchr/testify/mock"
)

// MockComprefaceClient is a mock implementation of the Compreface client
type MockComprefaceClient struct {
	mock.Mock
}

// DetectFaces mocks face detection
func (m *MockComprefaceClient) DetectFaces(imagePath string) (*compreface.DetectionResponse, error) {
	args := m.Called(imagePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*compreface.DetectionResponse), args.Error(1)
}

// RecognizeFace mocks face recognition
func (m *MockComprefaceClient) RecognizeFace(imagePath string) ([]compreface.RecognitionResult, error) {
	args := m.Called(imagePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]compreface.RecognitionResult), args.Error(1)
}

// AddSubject mocks subject creation
func (m *MockComprefaceClient) AddSubject(subjectName string, imagePath string) (*compreface.AddSubjectResponse, error) {
	args := m.Called(subjectName, imagePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*compreface.AddSubjectResponse), args.Error(1)
}

// ListSubjects mocks listing all subjects
func (m *MockComprefaceClient) ListSubjects() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// DeleteSubject mocks subject deletion
func (m *MockComprefaceClient) DeleteSubject(subjectName string) error {
	args := m.Called(subjectName)
	return args.Error(0)
}

// ListFaces mocks listing faces for a subject
func (m *MockComprefaceClient) ListFaces(subjectName string) (*compreface.FaceListResponse, error) {
	args := m.Called(subjectName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*compreface.FaceListResponse), args.Error(1)
}

// DeleteFace mocks face deletion
func (m *MockComprefaceClient) DeleteFace(imageID string) error {
	args := m.Called(imageID)
	return args.Error(0)
}
