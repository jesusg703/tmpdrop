package server

import (
	"sync"
	"time"
)

type pwAttempts struct {
	mu     sync.Mutex
	seen   map[string]*attemptRecord
	max    int
	window time.Duration
}

type attemptRecord struct {
	count int
	first time.Time
}

func newPWAttempts(max int, window time.Duration) *pwAttempts {
	return &pwAttempts{seen: map[string]*attemptRecord{}, max: max, window: window}
}

func (p *pwAttempts) allow(key string) bool {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, rec := range p.seen {
		if now.Sub(rec.first) > p.window {
			delete(p.seen, k)
		}
	}

	rec, ok := p.seen[key]
	if !ok || now.Sub(rec.first) > p.window {
		p.seen[key] = &attemptRecord{count: 1, first: now}
		return true
	}
	rec.count++
	return rec.count <= p.max
}

func (p *pwAttempts) reset(key string) {
	p.mu.Lock()
	delete(p.seen, key)
	p.mu.Unlock()
}
