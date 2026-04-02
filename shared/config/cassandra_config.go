package config

import (
	"log"
	"os"
	"time"

	"github.com/gocql/gocql"
)

type DbClient struct {
	Core    *gocql.Session
	Clinica *gocql.Session
}

func CassandraConnect() *DbClient {
	clusterCore := gocql.NewCluster(os.Getenv("CASSANDRA_IP_MASTER"))
	clusterCore.ProtoVersion = 4
	clusterCore.DisableInitialHostLookup = true
	clusterCore.IgnorePeerAddr = true
	clusterCore.Consistency = gocql.One
	clusterCore.Keyspace = os.Getenv("CASSANDRA_CORE_KEYSPACE")
	clusterCore.ConnectTimeout = 10 * time.Second
	clusterCore.Timeout = 10 * time.Second
	clusterCore.SocketKeepalive = 30 * time.Second

	sessionCore, err := clusterCore.CreateSession()
	if err != nil {
		log.Fatalf("❌ Erro na Matriz (AWS): %v", err)
	}

	clusterClinica := gocql.NewCluster(os.Getenv("CASSANDRA_IP_LOCAL"))
	clusterClinica.Keyspace = os.Getenv("CASSANDRA_CLINICA_KEYSPACE")
	clusterClinica.Consistency = gocql.One
	clusterClinica.ProtoVersion = 4
	clusterClinica.DisableInitialHostLookup = true
	clusterClinica.IgnorePeerAddr = true
	clusterClinica.ConnectTimeout = 10 * time.Second
	clusterClinica.Timeout = 10 * time.Second
	clusterClinica.SocketKeepalive = 30 * time.Second

	sessionClinica, err := clusterClinica.CreateSession()
	if err != nil {
		log.Printf("⚠️ Clínica Local offline: %v", err)
	}

	return &DbClient{
		Core:    sessionCore,
		Clinica: sessionClinica,
	}
}

func (db *DbClient) Close() {
	if db.Core != nil {
		db.Core.Close()
	}
	if db.Clinica != nil {
		db.Clinica.Close()
	}
}
