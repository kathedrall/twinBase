# MySQL TwinBase - Sistema de Migração Completa


<div align="left">




Um sistema de backup e migração de dados entre dois bancos MySQL, desenvolvido em Go. Este projeto permite realizar backup completo de schemas, estruturas de tabelas e dados de um banco MySQL origem para um banco MySQL destino, com suporte a processamento paralelo e bufferização inteligente.

![MySQL](https://img.shields.io/badge/MySQL-8.0+-blue?logo=mysql)

![Go](https://img.shields.io/badge/Go-1.26.4+-blue?logo=go)

![Docker](https://img.shields.io/badge/Docker-Compose-blue?logo=docker)

![License](https://img.shields.io/badge/License-MIT-green)

![Status](https://img.shields.io/badge/Status-Production%20Ready-brightgreen)
</div>


##  **Características Principais** 
- **Sanitização** de dados inválidos (datas 0000-00-00)
- **Buffer Inteligente**: Sistema de buffer configurável (padrão: 100.000 registros)



###  **Migração Completa**- **Progress Tracking**: Acompanhamento detalhado do progresso da migração

-  **Descoberta automática** de schemas, tabelas e estruturas

-  **Replicação inteligente** de estruturas com `IF NOT EXISTS` 

-  **Instalação Rápida- **Configuração Flexível**: Configuração via arquivo `.env`

-  **Migração de dados** com buffer configurável e processamento paralelo

-  **Migração de usuários** incluindo senhas, privilégios e configurações SSL- **CLI Intuitiva**: Interface de linha de comando fácil de usar



###  **Confiabilidade** 

-  **Sistema de checkpoint** para recuperação automática de erros

-  **Sanitização automática** de dados inválidos (datas 0000-00-00)

-  **Zero duplicação** de dados com verificação inteligente

-  **Dashboard visual** com progresso em tempo realgit clone <repo>


###  **Performance**cd twinBase- Go 1.24.6 ou superior

-  **Processamento paralelo** com goroutines configuráveis

-  **Buffer otimizado** até 100k registros por lote

- **Acesso a dois bancos MySQL** (origem e destino)

- **Continuação inteligente** exatamente onde parou

- **Monitoramento detalhado** com barras de progresso

- **Permissões apropriadas nos bancos de dados**

### Pré-requisitosExemplo de configuração `.env`:cd twinBase

- Go 1.24.6+

- MySQL 5.7+ ou 8.0 + ```bash```

- Docker (opcional, para ambiente de teste)

# MySQL Source Database (Database de origem)

### 1. Clone o projeto

```bash

git clone https://github.com/kathedrall/twinBase.git

cd twinBase

go mod tidy

go build -o twinBase main.go

./twinBase

```

### 2. Configure o ambiente
cp .env.example .env
# Edite .env com suas configurações

- ``MYSQL_SOURCE_HOST=192.168.0.20``
- ``MYSQL_SOURCE_PASSWORD=senha_origem``
- ``MYSQL_SOURCE_PORT=3306``
- ``MYSQL_SOURCE_USER=usuario_origem``
- ``MYSQL_SOURCE_DATABASE=petshop_db``

# MySQL Destination Database (Local Docker Container)  
- ``MYSQL_DEST_HOST=192.168.0.21``
- ``MYSQL_DEST_PORT=3307``
- ``MYSQL_DEST_USER=usuario_destino``
- ``MYSQL_DEST_PASSWORD=senha_odestino``
- ``MYSQL_DEST_DATABASE=petshop_db``

# Buffer Configuration
- ``BUFFER_SIZE=10000``   # Registros por lote
- ``MAX_GOROUTINES=10``   # Paralelismo

# Logging
- ``LOG_LEVEL=info``     # Modo Debug


### 4. Ambiente de teste (opcional)

Para facilitar os testes, o projeto inclui um ambiente de testes com dois containers de Mysql + phpMyadmin:
- **phpMyAdmin Origem**: `http://localhost:8080`
- **phpMyAdmin Destino**: `http://localhost:8081`


### Configurações Recomendadas

   Table: users (8 columns)

| Cenário | Buffer Size | Goroutines |

|---------|-------------|------------|   Table: products (12 columns)### Configuração Manual

| **Desenvolvimento** | 10.000 | 5 |

| **Produção Pequena** | 50.000 | 10 |   Table: orders (15 columns)

| **Produção Grande** | 100.000 | 20 |

| **Servidor Potente** | 200.000 | 50 | Schema: blog (2 tables)4. Edite o arquivo `.env` com suas configurações:

---   Table: posts (10 columns)```env

##  **Exemplos de Uso**

```bash

# 1. Descobrir estruturasMYSQL_SOURCE_PORT=3306

./twinBase discover

###  Replicação de EstruturasMYSQL_SOURCE_USER=root

# 2. Migração completa automática

./twinBase full  ***Cria schemas, tabelas e colunas no banco de destino (sem dados)

# 3. Migrar usuários

./twinBase users

# Continue a migração

./twinBase migrate **Migra apenas os dados (requer estrutura já criada)

```

### Monitoramento

```bash

# Verificar progresso
./twinBase progress

```


##  **Arquitetura**

```bash

twinBase/

├── main.go                    # CLI principal-  Sistema de checkpoint para recuperação

├── internal/

│   ├── config/               # Configurações-  Sanitização automática de datas inválidas

│   ├── database/             # Conexões MySQL

│   ├── models/               # Estruturas de dados

│   └── services/

│       ├── discovery.go      # Descoberta de estruturas

│       ├── replication.go    # Replicação de schemas

│       ├── migration.go      # Migração de dados

│       ├── user_migration.go # Migração de usuários

│       ├── checkpoint.go     # Sistema de checkpoint

│       └── sanitizer.go      # Limpeza de dados

├── docker-compose.yml        # Ambiente de teste


```

## Sistema de Checkpoint

```json** 
{

  "start_time": "2024-01-15T10:30:00Z",-  Usuários e senhas (hashes preservados) Descobre e lista todos os schemas, tabelas e estruturas do banco origem:

  "buffer_size": 100000,

  "tables": {-  Privilégios globais (SELECT, INSERT, UPDATE, etc.)

    "ecommerce.users": {

      "status": "completed",-  Privilégios por database```bash

      "processed_rows": 50000,

      "last_id": 50000-  Privilégios por tabela./twinBase discover

    }

  }-  Configurações de SSL e limites de recursos```

}

```



### Sanitização de Dados**Saída exemplo:**#### 2. Replicação de Estrutura

```sql

-- Corrige automaticamente:```Cria a estrutura (schemas, tabelas, índices) no banco destino:

'0000-00-00 00:00:00' → '2024-01-15 10:30:00'

NULL dates → Current timestamp User Migration Summary:

Invalid datetime → Valid datetime

```

### Dashboard Visual Users: 22 total, 22 migrated, 0 errors

```bash

 ecommerce.users: [████████████████████] 100.0% (50000/50000) Privileges: 49 total, 48 migrated, 1 errors```

 ecommerce.orders: [████████░░░░░░░░░░░░] 65.5% (32775/50000)

 Overall Progress: 150/744 tables completed (20.2%)==============================

```

### Áreas para Contribuição```

-  **Performance:** Otimizações de query e paralelismo

-  **Features:** Suporte a PostgreSQL, Oracle, etc.### 1. Subir containers

-  **Documentação:** Tutoriais e exemplos

-  **Testes:** Cobertura e casos edge```bash ### Exemplo de Uso Completo

-  **Bug Fixes:** Correções e melhorias

### Guidelines

- Código em inglês 
- Testes para novas funcionalidades
- Seguir padrões Go

Isso criará:

---

###  **v0.0.2 (Próxima Release)**

- [X] Migração de Triggers/Procedures

- [X] Suporte a PostgreSQL

- [X] Compressão de dados ./twinBase full

- [ ] Interface Web (UI)

- [ ] API REST


###  **v0.0.3 (Futuro)**### 

- [ ] Suporte a Oracle/SQL Server

- [ ] Suporte a Redis

- [ ] Clustering e distribuição

- [ ] Machine Learning para otimização


##  **Casos de Sucesso**

> *"Migrou 6TB de dados em 8 horas sem perder um registro"*

> *"Sistema de checkpoint salvou nossa migração de 3 dias"*  

> *"Facilidade de uso"*

> *"Open Source"*

##  **Agradecimentos**

- Tivit. Empresa que atuo por quase 4 anos

- Criadores e comunidade Golang pela linguagem incrível

- Equipe MySQL pelo banco robusto

- Docker pela containerização

- testadores


<div align="center"> 

