package services

import (
	"testing"
	"time"
)

func TestTakeChallengeIsSingleUseAndBoundToPatientAndAction(t *testing.T) {
	s := &PasskeyService{challenges: map[string]passkeyChallenge{}}
	id, err := s.storeChallenge(passkeyChallenge{patientID: "p1", purpose: purposeAction, action: "consent:grant:x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.takeChallenge(id, "p2", purposeAction, "consent:grant:x"); err == nil {
		t.Error("desafio de outro paciente foi aceito")
	}
	if _, err := s.takeChallenge(id, "p1", purposeAction, "consent:revoke:x"); err == nil {
		t.Error("desafio de outra ação foi aceito")
	}
	if _, err := s.takeChallenge(id, "p1", purposeAction, "consent:grant:x"); err == nil {
		t.Error("desafio reutilizado foi aceito")
	}
}

func TestTakeChallengeRejectsExpired(t *testing.T) {
	s := &PasskeyService{challenges: map[string]passkeyChallenge{
		"old": {patientID: "p1", purpose: purposeRegister, expires: time.Now().Add(-time.Second)},
	}}
	if _, err := s.takeChallenge("old", "p1", purposeRegister, ""); err == nil {
		t.Error("desafio expirado foi aceito")
	}
}
