# 📋 Checklist de Alinhamento Acadêmico — CONIC 2026
## Módulo 4: Barramento Centralizador de Dados Clínicos (Backend)

> **Documento Base:** *Artigo CONIC SEMESP 2026 — Plataforma SaaS Open Health (POHINC)*  
> **Repositório:** `sistema_centralizador_de_dados_clinicos_back`  
> **Tecnologias Centrais:** Go (Gin Gonic), Apache Cassandra, MariaDB, Keycloak IAM, HL7 FHIR Release 4, DPoP

---

### 📌 1. Visão Geral do Módulo no Artigo
Conforme descrito nas Seções **3.2, 4.2, 4.4, 5 e 6.2** do artigo:
* **Papel:** Barramento central analítico e transacional para interoperabilidade clínica no padrão **HL7 FHIR Release 4**.
* **Arquitetura de Dados:**
  - **Persistência Híbrida:** MariaDB para dados transacionais cadastrais e **Apache Cassandra** distribuído para escrita massiva de séries temporais de ECG e entidades FHIR desnormalizadas (Patient, Encounter, DiagnosticReport, Observation), indexadas por chaves compostas `(ID do Paciente, Timestamp do Exame)`.
  - **Validação Criptográfica DPoP:** Garantir que o acesso aos dados dos pacientes só ocorra mediante consentimento assinado em tempo real.
  - **Domínio de Produção Citado:** `https://central.pohinc.com.br`

---

### 🎯 2. Status Atual da Implementação
- [x] Servidor Go (Gin) com arquitetura modular (`services/users` e pasta `shared`).
- [x] Integração com MariaDB (GORM), Keycloak e Cassandra (`gocql`).
- [x] Middleware DPoP com verificação de replay e claims (`shared/dpop`).
- [x] Login de clínicas (`POST /api/auth/login`), busca de pacientes (`GET /api/patients/search`) e autorização com código OTP ou "Break the Glass" (`POST /api/patients/request-data`).
- [x] Serialização de recursos clínicos em HL7 FHIR Bundle (`GET /api/patients/:id/hl7-fhir`).

---

### ⏳ 3. Pendências e Itens Faltantes para Alinhamento com a Documentação

#### 3.1. Estruturação das Entidades FHIR no Apache Cassandra — 🚨 GAP CRÍTICO
- [ ] **Criar Tabelas de Recursos FHIR no Cassandra (`cassandra_migrations.go`):**
  - *Problema:* No código atual, o Cassandra possui apenas as tabelas `user_devices` e `register_logs`.
  - *O que o artigo exige:* Recursos clínicos estruturados em HL7 FHIR (Patient, Encounter, DiagnosticReport e Observation) e sinais densos de ECG devem ser persistidos no Cassandra de forma desnormalizada e autocontida.
  - *Ação:* Criar tabelas CQL em `shared/database/cassandra_migrations.go`:
    ```sql
    CREATE TABLE IF NOT EXISTS fhir_resources (
        patient_id uuid,
        exam_timestamp timestamp,
        resource_type text,
        resource_id text,
        fhir_payload text, -- JSON R4 autocontido
        PRIMARY KEY (patient_id, exam_timestamp, resource_id)
    ) WITH CLUSTERING ORDER BY (exam_timestamp DESC);
    ```

#### 3.2. Implementação das Rotas de Exames (`/api/exams`) para o App Móvel — 🚨 GAP CRÍTICO
- [ ] **Criar Controller e Rotas de Exames:**
  - O aplicativo móvel tenta consumir as seguintes rotas que atualmente **não existem** no backend:
    - `POST /api/exams`: upload de arquivo de exame (PDF/imagem) e metadados.
    - `GET /api/exams`: listagem dos exames pertencentes ao paciente autenticado via DPoP.
    - `GET /api/exams/:id`: detalhes de um exame específico.
    - `GET /api/exams/file/:id/:filename`: stream seguro para download do arquivo.
    - `DELETE /api/exams/:id`: marcação de exame como inativo (`flag_active = false`).
  - *Ação:* Registrar o grupo `/api/exams` no roteador em `services/users/cmd/main.go` conectando-o à tabela `exam` já existente no MariaDB.

#### 3.3. Efetivação do Compartilhamento de Exames e Consentimentos
- [ ] **Substituir o Mock em `user_handler.go` (`ShareExam`):**
  - Em `services/users/core/http/user_handler.go` (linha 470), o compartilhamento apenas registra uma linha de log genérica no Cassandra.
  - Gravar os registros reais de permissão na tabela `doctor_permission` com campos de identificação do médico, clínica autorizada, período de validade e hash do consentimento DPoP.

#### 3.4. Endpoint de Conexão com o Microsserviço de IA
- [ ] **Orquestração de Análise de ECG:**
  - Criar rota no centralizador (`POST /api/exams/:id/analyze-ecg`) que encaminha as matrizes numéricas do exame para o microserviço de IA (`inteligencia_artificial_para_analise_e_estudo_de_eletrocardiograma`) e armazena o resultado retornado no laudo do exame.
