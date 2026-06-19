package cache

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/service/flight-search/internal/model"
)

type entry struct {
	flights   []model.Flight
	metadata  model.Metadata
	expiresAt time.Time
}

// FlightCache provides an in-memory TTL cache for search results.
type FlightCache struct {
	mu      sync.RWMutex
	entries map[string]entry
	ttl     time.Duration
	done    chan struct{}
}

func New(ttl time.Duration) *FlightCache {
	c := &FlightCache{
		entries: make(map[string]entry),
		ttl:     ttl,
		done:    make(chan struct{}),
	}
	go c.cleanup()
	return c
}

func (c *FlightCache) Close() {
	close(c.done)
}

// Key generates a cache key from the search parameters.
func Key(req model.SearchRequest) string {
	raw := fmt.Sprintf("%s|%s|%s|%d|%s",
		req.Origin, req.Destination, req.DepartureDate,
		req.Passengers, req.CabinClass)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:8])
}

func (c *FlightCache) Get(key string) ([]model.Flight, model.Metadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, model.Metadata{}, false
	}
	return e.flights, e.metadata, true
}

func (c *FlightCache) Set(key string, flights []model.Flight, meta model.Metadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry{
		flights:   flights,
		metadata:  meta,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// cleanup removes expired entries every minute until Close() is called.
func (c *FlightCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, e := range c.entries {
				if now.After(e.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		case <-c.done:
			return
		}
	}
}
