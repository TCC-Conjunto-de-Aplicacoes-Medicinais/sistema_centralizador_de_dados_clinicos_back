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

func CassandraConnect(ips []string, localDC string, clinicaKeyspace string) *DbClient {
	
	cluster := gocql.NewCluster(ips...)
	cluster.Consistency = gocql.LocalQuorum
	cluster.Timeout = 5 * time.Second

	cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(localDC)
	cluster.RetryPolicy = &gocql.ExponentialBackoffRetryPolicy{NumRetries: 3}
	
	cluster.Keyspace = os.Getenv("CASSANDRA_CORE_KEYSPACE")
	sessionCore, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Error connecting to sistema_core: %v", err)
	}

	cluster.Keyspace = clinicaKeyspace
	sessionClinica, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Error connecting to %s: %v", clinicaKeyspace, err)
	}
	
	return &DbClient{
		Core: sessionCore,
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