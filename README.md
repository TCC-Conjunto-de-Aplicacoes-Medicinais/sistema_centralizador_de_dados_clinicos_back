# Sistema Centralizador de Dados Clínicos (Backend)

Este repositório contém o backend do **Sistema Centralizador de Dados Clínicos**, desenvolvido em Go (Golang). O sistema atua como o núcleo (core) de serviços de usuários, integrando segurança de nível militar, persistência poliglota distribuída, autenticação avançada OAuth2 com proteção de chaves e inteligência artificial para auxílio no diagnóstico médico.

---

## 🛠️ Tecnologias e Arquitetura

O sistema adota o padrão de microsserviço utilizando uma arquitetura robusta e escalável:

### 1. Engine Principal
*   **Linguagem**: Go (Golang 1.26+)
*   **Web Framework**: [Gin Gonic](https://github.com/gin-gonic/gin) para roteamento HTTP de alta performance.
*   **Documentação**: [Swagger](https://swagger.io/) via `swag` e `gin-swagger` para documentação viva e interativa da API.

### 2. Persistência Poliglota (Polyglot Persistence)
*   **MariaDB (Relacional - GORM)**: Utilizado para dados estruturados transacionais e relacionamentos fortes (ex: dados cadastrais dos pacientes, vínculos de médicos, clínicas e permissões de acesso).
*   **Apache Cassandra (NoSQL - gocql)**: Utilizado para dados de alto volume e gravação rápida que necessitam de distribuição em anel (ex: logs de auditoria detalhados e chaves criptográficas públicas dos dispositivos autorizados dos pacientes).

### 3. Autenticação e Segurança Avançada
*   **Identity Provider (IAM)**: Integração nativa com o **Keycloak** para gestão federada de identidades e fluxos OAuth2/OIDC.
*   **DPoP (Demonstrating Proof-of-Possession - RFC 9449)**: Mecanismo de segurança que vincula tokens de acesso a uma chave privada pertencente ao cliente. Impede o uso de tokens roubados ou vazados (reprodução).
*   **Anti-Replay Attack**: Validação de tempo de vida de DPoP proofs e detecção de repetição de JTI (*JWT ID*) via store em memória dedicada.

### 4. Inteligência Artificial
*   **Google Gemini API**: Utilização do modelo de linguagem avançado `gemini-2.0-flash` para fornecer segundas opiniões médicas baseadas em sintomas e exames digitados pelos pacientes (com o disclaimer obrigatório exigido por órgãos de saúde).

---

## 📁 Estrutura de Pastas do Projeto

```text
sistema_centralizador_de_dados_clinicos_back/
├── .github/workflows/      # Pipelines de CI/CD (GitHub Actions)
├── docs/                   # Documentação geral e arquitetural
├── scripts/                # Scripts auxiliares e de inicialização de DBs
├── services/               # Serviços / Componentes do Backend
│   └── users/              # Microsserviço de Usuários (Pacientes)
│       ├── cmd/            # Ponto de entrada do serviço (main.go, docs do swagger)
│       └── core/           # Núcleo de regras de negócio
│           ├── http/       # Handlers, rotas e middlewares (DPoP, Auth)
│           ├── services/   # Regras de negócio (IA, Login, Signup, VerifyEmail)
│           └── usecase/    # Casos de uso de validações de regras
├── shared/                 # Código e configurações compartilhadas
│   ├── auth/               # Helpers de autenticação
│   ├── config/             # Conexões e cargas de configurações de infraestrutura
│   ├── database/           # Schemas, migrações e modelos do GORM/Cassandra
│   ├── dpop/               # Biblioteca de validação de DPoP proofs (RFC 9449)
│   ├── logger/             # Biblioteca de gravação de logs de auditoria no Cassandra
│   └── models/             # Modelos de requisição e resposta expostos pela API
└── tests/                  # Testes automatizados
    └── unitTests/          # Suíte de testes unitários (Handler, Service, Usecase)
```

---

## 🔑 Configuração de Ambiente (`.env`)

Copie o arquivo `.env.example` para `.env` e defina as variáveis necessárias antes de rodar o serviço:

```bash
cp .env.example .env
```

| Variável | Descrição |
| :--- | :--- |
| `CASSANDRA_IP_LOCAL` | IP local para o cluster Cassandra local |
| `CASSANDRA_IP_MASTER` | IP do Cassandra mestre (ou do container do Docker Compose) |
| `CASSANDRA_CORE_KEYSPACE` | Keyspace do Cassandra para os dados do Core (Padrão: `sistema_core`) |
| `CASSANDRA_LOCAL_DC` | Identificação do Data Center local no Cassandra (Ex: `datacenter1`) |
| `KEYCLOAK_URL` | URL do servidor Keycloak (Ex: `http://localhost:8080`) |
| `KEYCLOAK_CLIENT_ID` | Client ID do Realm no Keycloak |
| `KEYCLOAK_CLIENT_SECRET` | Client Secret do Keycloak para autorizações confidenciais |
| `KEYCLOAK_REALM` | Nome do Realm configurado no Keycloak |
| `MARIADB_HOST` | Host do banco MariaDB |
| `MARIADB_PORT` | Porta de conexão do MariaDB (Padrão: `3306`) |
| `MARIADB_USER` | Usuário do MariaDB |
| `MARIADB_PASSWORD` | Senha do MariaDB |
| `MARIADB_DB` | Nome do banco relacional principal |
| `BASE_URL` | URL base desta API (necessária para validação da claim `htu` do DPoP) |
| `SMTP_HOST` | Servidor SMTP para envio de e-mails de validação |
| `SMTP_PORT` | Porta do servidor SMTP |
| `SMTP_USER` | Usuário de autenticação SMTP |
| `SMTP_PASSWORD` | Senha de autenticação SMTP |
| `GEMINI_API_KEY` | Chave de acesso à API Google Gemini AI |

---

## 🗺️ Tabela de Endpoints da API

Todas as rotas da API possuem o prefixo `/api`. As requisições que exigem **DPoP** necessitam do cabeçalho HTTP `DPoP` contendo um JWT assinado com a respectiva chave pública do dispositivo do usuário.

| Método | Endpoint | Proteção | Descrição |
| :---: | :--- | :---: | :--- |
| `POST` | `/api/signup` | Pública | Cadastra um paciente integrando Keycloak, MariaDB e Cassandra. |
| `POST` | `/api/login` | DPoP | Autentica um paciente via Keycloak e retorna tokens DPoP-bound. |
| `POST` | `/api/refresh` | DPoP | Emite um novo token usando o RefreshToken e DPoP proof. |
| `GET` | `/api/users/profile` | Auth + DPoP | Retorna dados editáveis do perfil do paciente. |
| `PUT` | `/api/users` | Auth + DPoP | Atualiza dados cadastrais do paciente (nome, telefone, endereço). |
| `POST` | `/api/users/send-verify-email` | Auth + DPoP | Dispara um código de confirmação ao e-mail do usuário. |
| `POST` | `/api/users/verify-email-code` | Auth + DPoP | Valida o código de confirmação enviado ao e-mail. |
| `POST` | `/api/users/exams/share` | Auth + DPoP | Compartilha um exame de forma auditada no Cassandra. |
| `POST` | `/api/ai/analyze` | Auth + DPoP | Solicita uma análise inteligente de exames/sintomas ao Gemini. |
| `GET` | `/swagger/*any` | Pública | Acesso à interface gráfica Swagger UI para testes rápidos. |

---

## 🚀 Como Executar o Projeto

### Pré-requisitos
*   [Docker](https://www.docker.com/) e Docker Compose instalados.
*   [Go 1.26+](https://go.dev/) (opcional, para execução local direta).

### Passo 1: Subir os serviços auxiliares via Docker Compose
O comando abaixo irá provisionar e configurar de forma integrada os containers de MariaDB, Apache Cassandra e Keycloak:

```bash
docker-compose up -d
```

### Passo 2: Executar o Microsserviço de Usuários localmente
Certifique-se de configurar as variáveis de conexão apropriadas no seu arquivo `.env` para apontar ao `localhost` (ou os IPs expostos das portas).

Instale as dependências e rode a API:
```bash
go mod tidy
go run services/users/cmd/main.go
```
O serviço iniciará escutando na porta **`8002`** (conforme registrado no main).

---

## 🧪 Testes Automatizados e Cobertura

O projeto adota uma política estrita de cobertura com testes unitários cobrindo o core de lógica (handlers, services e usecases).

### Executar a suíte de testes unitários:
```bash
go test ./tests/unitTests -v -count=1
```

### Gerar e verificar relatório de cobertura de código:
1.  Gere o arquivo de cobertura:
    ```bash
    go test -count=1 -coverpkg=./services/users/core/... ./tests/unitTests -coverprofile=coverage.out
    ```
2.  Veja a cobertura de funções detalhada no terminal:
    ```bash
    go tool cover -func=coverage.out
    ```
3.  (Opcional) Visualize a cobertura graficamente no navegador:
    ```bash
    go tool cover -html=coverage.out
    ```

---

## 📝 Atualização da Documentação Swagger

Caso modifique alguma rota, structs expostas ou anotações Swagger, você precisará gerar os arquivos de especificação novamente.

Com o utilitário `swag` instalado na máquina, execute na raiz do projeto:
```bash
swag init -g services/users/cmd/main.go -o services/users/cmd/docs
```
Isso atualizará os esquemas JSON/YAML expostos na rota `/swagger/index.html`.