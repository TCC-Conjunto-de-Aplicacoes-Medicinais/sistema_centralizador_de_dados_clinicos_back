package http

import (
	"strings"
	"sync"
	"time"
)

// loginLimiter bloqueia temporariamente um e-mail depois de N senhas erradas seguidas.
// O estado fica em memória: basta para uma instância do back. Com várias instâncias, o
// contador precisaria ir para um armazenamento compartilhado.
type loginLimiter struct {
	mu          sync.Mutex
	maxFailures int
	lockFor     time.Duration
	entries     map[string]*loginAttempts
}

type loginAttempts struct {
	failures    int
	lockedUntil time.Time
}

func newLoginLimiter(maxFailures int, lockFor time.Duration) *loginLimiter {
	return &loginLimiter{maxFailures: maxFailures, lockFor: lockFor, entries: map[string]*loginAttempts{}}
}

func normalizeLoginKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// blockedFor devolve quanto tempo falta para o desbloqueio (0 se não está bloqueado).
func (l *loginLimiter) blockedFor(email string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[normalizeLoginKey(email)]
	if !ok || !now.Before(e.lockedUntil) {
		return 0
	}
	return e.lockedUntil.Sub(now)
}

// recordFailure conta uma senha errada e bloqueia ao atingir o limite.
func (l *loginLimiter) recordFailure(email string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := normalizeLoginKey(email)
	e, ok := l.entries[key]
	if !ok {
		e = &loginAttempts{}
		l.entries[key] = e
	}
	e.failures++
	if e.failures >= l.maxFailures {
		e.lockedUntil = now.Add(l.lockFor)
		e.failures = 0
	}
}

// reset zera o contador após um login bem-sucedido.
func (l *loginLimiter) reset(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, normalizeLoginKey(email))
}
