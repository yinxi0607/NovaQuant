package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

type Registry struct {
	mu       sync.RWMutex
	counters map[string]*atomic.Uint64
}

func New() *Registry {
	return &Registry{counters: map[string]*atomic.Uint64{}}
}

func (r *Registry) Inc(name string) {
	r.mu.RLock()
	counter, ok := r.counters[name]
	r.mu.RUnlock()
	if !ok {
		r.mu.Lock()
		counter = r.counters[name]
		if counter == nil {
			counter = &atomic.Uint64{}
			r.counters[name] = counter
		}
		r.mu.Unlock()
	}
	counter.Add(1)
}

func (r *Registry) Add(name string, delta uint64) {
	r.mu.RLock()
	counter, ok := r.counters[name]
	r.mu.RUnlock()
	if !ok {
		r.mu.Lock()
		counter = r.counters[name]
		if counter == nil {
			counter = &atomic.Uint64{}
			r.counters[name] = counter
		}
		r.mu.Unlock()
	}
	counter.Add(delta)
}

func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		r.mu.RLock()
		names := make([]string, 0, len(r.counters))
		for name := range r.counters {
			names = append(names, name)
		}
		r.mu.RUnlock()
		sort.Strings(names)
		for _, name := range names {
			r.mu.RLock()
			counter := r.counters[name]
			r.mu.RUnlock()
			_, _ = fmt.Fprintf(w, "# TYPE %s counter\n%s %d\n", name, name, counter.Load())
		}
	}
}
