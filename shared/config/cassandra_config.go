package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

type DbClient struct {
	Core *gocql.Session
}

func CassandraConnect() *DbClient {
	var nodes []string
	if ipLocal := os.Getenv("CASSANDRA_IP_LOCAL"); ipLocal != "" {
		nodes = append(nodes, ipLocal)
	}
	if ipMaster := os.Getenv("CASSANDRA_IP_MASTER"); ipMaster != "" {
		nodes = append(nodes, ipMaster)
	}

	if len(nodes) == 0 {
		log.Println("⚠️ Nenhum nó do Cassandra fornecido nas variáveis de ambiente.")
	}

	cluster := gocql.NewCluster(nodes...)
	cluster.ProtoVersion = 4

	cluster.Keyspace = os.Getenv("CASSANDRA_CORE_KEYSPACE")

	localDC := strings.TrimSpace(os.Getenv("CASSANDRA_LOCAL_DC"))
	if localDC != "" {
		cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(localDC)
	}

	cluster.Consistency = gocql.LocalOne

	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 10 * time.Second
	cluster.SocketKeepalive = 30 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("❌ Erro fatal ao ingressar no Cluster do Cassandra: %v", err)
	}

	if localDC != "" {
		log.Printf("✅ Cassandra aderido! Datacenter ativo: [%s] | IPs detectados: %v", localDC, nodes)
	} else {
		log.Printf("✅ Cassandra aderido na Matriz Core (Balanceamento global) | IPs detectados: %v", nodes)
	}

	return &DbClient{
		Core: session,
	}
}

func (db *DbClient) Close() {
	if db.Core != nil {
		db.Core.Close()
	}
}
