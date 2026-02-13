package service

import (
	"context"
	"fmt"
	"sync"
)

// MockOrganizationService provides a mock implementation of OrganizationService.
type MockOrganizationService struct {
	superiors   map[string]struct{ id, name string }
	deptLeaders map[string][]string
}

func (m *MockOrganizationService) GetSuperior(_ context.Context, userID string) (string, string, error) {
	if s, ok := m.superiors[userID]; ok {
		return s.id, s.name, nil
	}

	return "", "", nil
}

func (m *MockOrganizationService) GetDeptLeaders(_ context.Context, deptID string) ([]string, error) {
	if leaders, ok := m.deptLeaders[deptID]; ok {
		return leaders, nil
	}

	return nil, nil
}

// MockUserService provides a mock implementation of UserService.
type MockUserService struct {
	roleUsers map[string][]string
}

func (m *MockUserService) GetUsersByRole(_ context.Context, roleID string) ([]string, error) {
	if users, ok := m.roleUsers[roleID]; ok {
		return users, nil
	}

	return nil, nil
}

// MockSerialNoGenerator provides a mock implementation of SerialNoGenerator.
type MockSerialNoGenerator struct {
	mu      sync.Mutex
	counter map[string]int
}

func NewMockSerialNoGenerator() *MockSerialNoGenerator {
	return &MockSerialNoGenerator{counter: make(map[string]int)}
}

func (m *MockSerialNoGenerator) Generate(_ context.Context, flowCode string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter[flowCode]++

	return fmt.Sprintf("%s-%04d", flowCode, m.counter[flowCode]), nil
}
