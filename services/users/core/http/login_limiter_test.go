package http

import (
	"testing"
	"time"
)

func TestLoginLimiter_BlocksAfterThreeFailures(t *testing.T) {
	l := newLoginLimiter(3, 15*time.Minute)
	now := time.Now()
	email := "clinica@example.com"

	l.recordFailure(email, now)
	l.recordFailure(email, now)
	if l.blockedFor(email, now) != 0 {
		t.Fatal("não deveria bloquear antes da 3ª falha")
	}
	l.recordFailure(email, now)
	if l.blockedFor(email, now) <= 0 {
		t.Fatal("deveria bloquear após a 3ª falha")
	}
}

func TestLoginLimiter_UnblocksAfterWindow(t *testing.T) {
	l := newLoginLimiter(3, 15*time.Minute)
	now := time.Now()
	for i := 0; i < 3; i++ {
		l.recordFailure("a@example.com", now)
	}
	if l.blockedFor("a@example.com", now.Add(16*time.Minute)) != 0 {
		t.Fatal("bloqueio deveria expirar depois de 15 minutos")
	}
}

func TestLoginLimiter_ResetOnSuccessAndCaseInsensitive(t *testing.T) {
	l := newLoginLimiter(3, 15*time.Minute)
	now := time.Now()
	l.recordFailure("Clinica@Example.com", now)
	l.recordFailure("clinica@example.com", now)
	l.reset(" CLINICA@example.com ")
	l.recordFailure("clinica@example.com", now)
	if l.blockedFor("clinica@example.com", now) != 0 {
		t.Fatal("login bem-sucedido deveria zerar o contador, independente de maiúsculas")
	}
}
