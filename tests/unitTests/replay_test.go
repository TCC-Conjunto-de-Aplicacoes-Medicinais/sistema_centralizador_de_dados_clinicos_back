package unitTests

import (
	"testing"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/dpop"
	"github.com/stretchr/testify/assert"
)

func TestReplayStore_CheckAndStore(t *testing.T) {
	store := dpop.NewReplayStore(100 * time.Millisecond)

	err := store.CheckAndStore("jti-123")
	assert.NoError(t, err)

	err = store.CheckAndStore("jti-123")
	assert.Error(t, err)
	assert.Equal(t, "dpop: jti já utilizado (replay detectado)", err.Error())

	time.Sleep(150 * time.Millisecond)
	err = store.CheckAndStore("jti-123") 
	assert.NoError(t, err)
}

func TestReplayStore_Purge(t *testing.T) {
	store := dpop.NewReplayStore(10 * time.Millisecond)
	
	store.CheckAndStore("jti-purge")

	time.Sleep(30 * time.Millisecond)
	
	err := store.CheckAndStore("jti-purge")
	assert.NoError(t, err)
}
