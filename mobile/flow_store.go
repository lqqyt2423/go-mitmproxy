package mobile

import (
	"fmt"
	"sync"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

// flowStore holds flow references for on-demand body retrieval from the Swift side.
// Bodies are not sent automatically via EventHandler to avoid large data transfers.
type flowStore struct {
	mu    sync.RWMutex
	flows map[string]*proxy.Flow
	order []string // insertion order for eviction
	limit int
}

func newFlowStore(limit int) *flowStore {
	return &flowStore{
		flows: make(map[string]*proxy.Flow),
		order: make([]string, 0, limit),
		limit: limit,
	}
}

func (s *flowStore) Put(f *proxy.Flow) {
	id := f.Id.String()
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.flows[id]; exists {
		return
	}

	s.flows[id] = f
	s.order = append(s.order, id)

	for len(s.flows) > s.limit && len(s.order) > 0 {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.flows, oldest)
	}
}

func (s *flowStore) Get(id string) (*proxy.Flow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flows[id]
	if !ok {
		return nil, fmt.Errorf("flow %s not found", id)
	}
	return f, nil
}
