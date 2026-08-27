package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

type ObjectStore struct {
	mu        sync.RWMutex
	objects   map[string][]byte
	maxObject int64
}

func NewObjectStore(max int64) *ObjectStore {
	if max < 1 {
		max = 64 << 20
	}
	return &ObjectStore{objects: map[string][]byte{}, maxObject: max}
}
func (s *ObjectStore) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if size < 0 || size > s.maxObject {
		return fmt.Errorf("object size %d outside bounds", size)
	}
	data, err := io.ReadAll(io.LimitReader(r, s.maxObject+1))
	if err != nil {
		return fmt.Errorf("read object: %w", err)
	}
	if int64(len(data)) > s.maxObject {
		return fmt.Errorf("object exceeds limit")
	}
	s.mu.Lock()
	s.objects[key] = append([]byte(nil), data...)
	s.mu.Unlock()
	return nil
}
func (s *ObjectStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	data, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("object %s not found", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (s *ObjectStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}
