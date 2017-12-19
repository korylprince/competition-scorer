package api

import (
	"sync"
	"time"
)

//Session represents a login session
type Session struct {
	Expires time.Time
}

//MemorySessionStore represents a SessionStore that uses an in-memory map
type MemorySessionStore struct {
	store    map[string]*Session
	duration time.Duration
	mu       *sync.Mutex
}

//scavenge removes stale records every hour
func scavenge(m *MemorySessionStore) {
	for {
		time.Sleep(time.Hour)
		now := time.Now()
		m.mu.Lock()
		for id, t := range m.store {
			if t.Expires.Before(now) {
				delete(m.store, id)
			}
		}
		m.mu.Unlock()
	}
}

//NewMemorySessionStore returns a new MemorySessionStore with the given expiration duration.
func NewMemorySessionStore(duration time.Duration) *MemorySessionStore {
	m := &MemorySessionStore{
		store:    make(map[string]*Session),
		duration: duration,
		mu:       new(sync.Mutex),
	}
	go scavenge(m)
	return m
}

//Create returns a new sessionID
func (m *MemorySessionStore) Create() string {
	id := randString(22)
	m.mu.Lock()
	m.store[id] = &Session{
		Expires: time.Now().Add(m.duration),
	}
	m.mu.Unlock()
	return id
}

//Check returns whether or not sessionID is a valid session
func (m *MemorySessionStore) Check(sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.store[sessionID]; ok {
		if s.Expires.After(time.Now()) {
			s.Expires = time.Now().Add(m.duration)
			return true
		}
		delete(m.store, sessionID)
	}
	return false
}
