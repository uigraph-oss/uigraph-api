package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// testStorage is a package-level in-memory storage instance shared across tests.
var testStorage = newMockStorage()

type mockStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{objects: make(map[string][]byte)}
}

func (m *mockStorage) EnsureBucket(_ context.Context) error { return nil }

func (m *mockStorage) Upload(_ context.Context, key, _ string, r io.Reader, _ int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.objects[key] = data
	m.mu.Unlock()
	return nil
}

func (m *mockStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	data, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("storage: key not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.objects, key)
	m.mu.Unlock()
	return nil
}

func (m *mockStorage) PresignURL(_ context.Context, key string) (string, error) {
	return "http://mock-storage/" + key, nil
}
