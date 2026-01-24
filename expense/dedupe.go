package expense

import (
	"sync"
	"time"
)

type Deduper struct {
	mu     sync.Mutex
	window time.Duration
	cache  map[string]time.Time
}

func NewDeduper(window time.Duration) *Deduper {
	return &Deduper{
		window: window,
		cache:  make(map[string]time.Time),
	}
}

func (d *Deduper) Seen(key string) bool {

	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	if last, ok := d.cache[key]; ok {

		if now.Sub(last) < d.window {
			return true
		}
	}

	d.cache[key] = now
	return false
}
