package shared

import "sync"

type Set[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

func NewSet[K comparable, V any]() *Set[K, V] {
	return &Set[K, V]{items: make(map[K]V)}
}

func (s *Set[K, V]) Add(key K, value V) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[key]; exists {
		return false
	}
	s.items[key] = value
	return true
}

func (s *Set[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.items[key]
	return value, ok
}

func (s *Set[K, V]) Remove(key K) (V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	delete(s.items, key)
	return value, true
}

func (s *Set[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items)
}

func (s *Set[K, V]) List() []V {
	s.mu.RLock()
	defer s.mu.RUnlock()

	values := make([]V, 0, len(s.items))
	for _, value := range s.items {
		values = append(values, value)
	}
	return values
}

func (s *Set[K, V]) Clear() []V {
	s.mu.Lock()
	defer s.mu.Unlock()

	values := make([]V, 0, len(s.items))
	for _, value := range s.items {
		values = append(values, value)
	}
	s.items = make(map[K]V)
	return values
}
