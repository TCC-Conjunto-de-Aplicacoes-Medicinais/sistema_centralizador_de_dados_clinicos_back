package config

import (
	"fmt"
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


	localDC := strings.TrimSpace(os.Getenv("CASSANDRA_LOCAL_DC"))
	if localDC != "" {
		cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(localDC)
	}

	cluster.Consistency = gocql.LocalOne

	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 10 * time.Second
	cluster.SocketKeepalive = 30 * time.Second

	// 1. Criar uma sessão inicial (sem keyspace) para poder criá-lo
	tempSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("❌ Erro fatal ao conectar no Cassandra para setup: %v", err)
	}

	keyspace := os.Getenv("CASSANDRA_CORE_KEYSPACE")
	if keyspace == "" {
		keyspace = "sistema_core"
	}

	// 2. Criar o keyspace se não existir
	createKsQuery := fmt.Sprintf(`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};`, keyspace)
	if err := tempSession.Query(createKsQuery).Exec(); err != nil {
		log.Fatalf("❌ Erro ao criar Keyspace '%s': %v", keyspace, err)
	}

	tempSession.Close()

	// 4. Agora sim, conectar especificando o Keyspace
	cluster.Keyspace = keyspace
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
