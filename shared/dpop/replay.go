package dpop

import (
	"errors"
	"sync"
	"time"
)

type jtiEntry struct {
	expiresAt time.Time
}

// ReplayStore previne reutilização de DPoP proofs via controle de JTI.
// Implementação in-memory com TTL; para multi-instância, substituir por Redis.
type ReplayStore struct {
	mu  sync.Map
	ttl time.Duration
}

func NewReplayStore(ttl time.Duration) *ReplayStore {
	store := &ReplayStore{ttl: ttl}
	go store.purge()
	return store
}

// CheckAndStore retorna erro se o JTI já foi usado e ainda está dentro do TTL.
// Operação atômica via LoadOrStore.
func (rs *ReplayStore) CheckAndStore(jti string) error {
	now := time.Now()
	newEntry := &jtiEntry{expiresAt: now.Add(rs.ttl)}

	actual, loaded := rs.mu.LoadOrStore(jti, newEntry)
	if loaded {
		existing := actual.(*jtiEntry)
		if existing.expiresAt.After(now) {
			return errors.New("dpop: jti já utilizado (replay detectado)")
		}
		// Entrada expirada — substituir
		rs.mu.Store(jti, newEntry)
	}
	return nil
}

func (rs *ReplayStore) purge() {
	ticker := time.NewTicker(rs.ttl)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rs.mu.Range(func(k, v interface{}) bool {
			if v.(*jtiEntry).expiresAt.Before(now) {
				rs.mu.Delete(k)
			}
			return true
		})
	}
}
