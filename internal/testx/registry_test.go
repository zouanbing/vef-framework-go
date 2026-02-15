package testx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type mockBase struct {
	value string
}

type MockFeatureTestSuite struct {
	suite.Suite
	base *mockBase
}

func (s *MockFeatureTestSuite) TestPlaceholder() {
	s.Equal("test", s.base.value)
}

// TestRegistryAdd tests registry add functionality.
func TestRegistryAdd(t *testing.T) {
	r := NewRegistry[mockBase]()

	r.Add(func(base *mockBase) suite.TestingSuite {
		return &MockFeatureTestSuite{base: base}
	})

	assert.Equal(t, 1, r.Len(), "Registry should have 1 factory")
}

// TestRegistryAddNamed tests registry add named functionality.
func TestRegistryAddNamed(t *testing.T) {
	r := NewRegistry[mockBase]()

	r.AddNamed("CustomName", func(base *mockBase) suite.TestingSuite {
		return &MockFeatureTestSuite{base: base}
	})

	assert.Equal(t, 1, r.Len(), "Registry should have 1 factory")
}

// TestRegistryNameExtraction tests registry name extraction functionality.
func TestRegistryNameExtraction(t *testing.T) {
	r := NewRegistry[mockBase]()

	r.Add(func(base *mockBase) suite.TestingSuite {
		return &MockFeatureTestSuite{base: base}
	})

	// Verify the name was extracted correctly (MockFeatureTestSuite → MockFeature)
	assert.Equal(t, "MockFeature", r.factories[0].name, "Should strip TestSuite suffix")
}
