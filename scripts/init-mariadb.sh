#!/bin/bash
# Script para criar bancos adicionais no MariaDB usando variáveis de ambiente
set -e

mysql -u root -p"${MARIADB_ROOT_PASSWORD}" <<-EOSQL
    CREATE DATABASE IF NOT EXISTS keycloak_db;
    GRANT ALL PRIVILEGES ON keycloak_db.* TO '${MARIADB_USER}'@'%';
    FLUSH PRIVILEGES;
EOSQL
