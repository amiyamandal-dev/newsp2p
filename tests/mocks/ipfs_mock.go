package mocks

import (
	"context"
	"errors"
	"sync"
)

// MockIPFSClient implements IPFSClient interface for testing
type MockIPFSClient struct {
	mu      sync.Mutex
	Storage map[string][]byte
}

func NewMockIPFSClient() *MockIPFSClient {
	return &MockIPFSClient{
		Storage: make(map[string][]byte),
	}
}

func (m *MockIPFSClient) Add(ctx context.Context, data []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple mock CID generation (not real multihash)
	// In real tests we might want something deterministic
	cid := "QmMockCID" + string(data)[:5] // Use first 5 bytes for variance
	if len(data) > 5 {
		cid = "QmMockCID" + string(data[:5])
	}
	
	m.Storage[cid] = data
	return cid, nil
}

func (m *MockIPFSClient) Cat(ctx context.Context, cid string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, exists := m.Storage[cid]
	if !exists {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (m *MockIPFSClient) Unpin(ctx context.Context, cid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Mock unpin always succeeds
	return nil
}
