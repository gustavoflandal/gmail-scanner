# 📧 Gmail Scanner

**Extrator inteligente de artigos de newsletters do Gmail**

Uma aplicação completa para escanear emails do Gmail via IMAP, extrair links de newsletters e criar uma lista de leitura offline. Interface web moderna com React, backend em Go e armazenamento em SQLite + BBolt.

---

## ✨ Recursos

- 🔐 **Autenticação IMAP** - Login com email + senha de app do Google
- 📁 **Seleção de Pastas** - Escolha quais pastas escanear (INBOX, Newsletters, etc)
- 📊 **Progresso em Tempo Real** - Acompanhe a varredura com barra de progresso
- ⏹️ **Cancelamento** - Interrompa a varredura a qualquer momento
- 📰 **Extração de Artigos** - Extrai links e títulos de newsletters
- 📖 **Lista de Leitura** - Importe artigos para leitura offline com conteúdo completo
- 🔍 **Busca e Filtros** - Filtre por newsletter, domínio ou texto
- 🐳 **Docker Ready** - Deploy com um comando

---

## 🚀 Início Rápido

### Pré-requisitos

- **Docker** e **Docker Compose**
- **Conta Gmail** com:
  - IMAP habilitado
  - Verificação em 2 etapas ativada
  - Senha de App gerada

### 1. Clonar e Iniciar

```bash
# Clonar repositório
git clone https://github.com/gustavoflandal/Gmail-Scanner.git
cd Gmail-Scanner

# Iniciar com Docker
docker-compose up --build -d

# Verificar status
docker ps
```

### 2. Acessar Aplicação

Abra no navegador: **http://localhost:8080**

### 3. Fazer Login

1. Clique em **"Login"**
2. Digite seu email do Gmail
3. Use sua **Senha de App** (não a senha normal!)

---

## 🔑 Gerar Senha de App do Google

1. Acesse [myaccount.google.com/security](https://myaccount.google.com/security)
2. Ative a **Verificação em 2 etapas** (se ainda não tiver)
3. Acesse [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords)
4. Selecione "Outro" e digite "Gmail Scanner"
5. Copie a senha de 16 caracteres gerada
6. Use essa senha no login da aplicação

---

## 📡 API Endpoints

### Autenticação
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| POST | `/api/auth/login` | Login com email + senha IMAP |
| POST | `/api/auth/logout` | Logout |

### Varredura
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| POST | `/api/scan` | Inicia varredura `{"folders": ["INBOX"]}` |
| POST | `/api/scan-cancel` | Cancela varredura em andamento |
| GET | `/api/scan-status` | Status da varredura |
| GET | `/api/scan-progress` | Progresso detalhado |
| GET | `/api/folders` | Lista pastas IMAP disponíveis |

### Artigos
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| GET | `/api/articles` | Lista artigos extraídos |
| DELETE | `/api/articles/{id}` | Remove artigo |
| GET | `/api/newsletters` | Lista newsletters encontradas |

### Lista de Leitura
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| POST | `/api/reading-list/import` | Importa artigo com conteúdo |
| GET | `/api/reading-list` | Lista artigos importados |
| GET | `/api/reading-list/{id}` | Obtém artigo com conteúdo |
| DELETE | `/api/reading-list/{id}` | Remove da lista de leitura |

### Sistema
| Método | Endpoint | Descrição |
|--------|----------|-----------|
| GET | `/api/health` | Health check |
| GET | `/api/stats` | Estatísticas do banco |

---

## 📁 Estrutura do Projeto

```
Gmail-Scanner/
├── cmd/api/
│   └── main.go              # Servidor HTTP + handlers
├── internal/
│   ├── auth/
│   │   └── simple.go        # Autenticação JWT + IMAP
│   ├── database/
│   │   └── db.go            # SQLite (artigos)
│   ├── imap/
│   │   └── client.go        # Cliente IMAP
│   ├── nosql/
│   │   └── nosql.go         # BBolt (lista de leitura)
│   └── scraper/
│       └── scraper.go       # Extração de conteúdo
├── web/
│   ├── src/
│   │   ├── pages/
│   │   │   ├── Dashboard.jsx
│   │   │   ├── Articles.jsx
│   │   │   ├── ReadArticle.jsx
│   │   │   └── Login.jsx
│   │   ├── components/
│   │   ├── services/
│   │   │   └── api.js
│   │   └── utils/
│   │       └── storage.js
│   ├── package.json
│   └── vite.config.js
├── docker-compose.yml
├── Dockerfile
├── .env.example
└── README.md
```

---

## ⚙️ Configuração

### Variáveis de Ambiente (.env)

```env
# Aplicação
APP_ENV=development
LOG_LEVEL=info

# JWT (mude em produção!)
JWT_SECRET=sua-chave-secreta-aqui

# IMAP Gmail (padrão)
IMAP_HOST=imap.gmail.com
IMAP_PORT=993
```

### Docker Compose

```yaml
services:
  gmail-scanner:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - gmail-data:/app/data
    environment:
      - JWT_SECRET=${JWT_SECRET:-change-me}
```

---

## 🐳 Comandos Docker

```bash
# Iniciar
docker-compose up -d

# Rebuild e iniciar
docker-compose up --build -d

# Ver logs
docker-compose logs -f gmail-scanner

# Parar
docker-compose down

# Remover tudo (incluindo volumes)
docker-compose down -v
```

---

## 💻 Desenvolvimento Local

### Backend (Go)

```bash
# Instalar dependências
go mod download

# Executar
go run ./cmd/api/main.go
```

### Frontend (React)

```bash
cd web

# Instalar dependências
npm install

# Executar em modo dev
npm run dev

# Build para produção
npm run build
```

---

## 🗄️ Arquitetura de Dados

O sistema utiliza **dois bancos de dados** com propósitos distintos:

```
📧 Email com newsletter
       ↓
   [Varredura IMAP]
       ↓
📋 Links extraídos → SQLite (emails.db)
       ↓
   [Usuário clica "Importar"]
       ↓
📖 Artigo completo → BBolt (reading_list.db)
```

### SQLite (`emails.db`) - Artigos Extraídos

| Campo | Descrição |
|-------|-----------|
| `url` | URL do artigo (UNIQUE - evita duplicatas) |
| `title` | Título extraído do link |
| `description` | Descrição/resumo |
| `domain` | Domínio do site |
| `newsletter` | Nome da newsletter (remetente) |
| `email_date` | Data do email original |
| `folder` | Pasta IMAP de origem |

**Características:**
- Armazena **links** encontrados durante a varredura
- URLs são **normalizadas** (parâmetros de tracking removidos)
- Índice UNIQUE na URL impede duplicatas
- Usa `INSERT OR IGNORE` para performance

```bash
# Acessar banco no container
docker exec -it gmail-scanner sh
sqlite3 /app/data/emails.db

# Consultas úteis
SELECT COUNT(*) FROM articles;
SELECT DISTINCT newsletter FROM articles;
SELECT domain, COUNT(*) as total FROM articles GROUP BY domain ORDER BY total DESC;
```

### BBolt (`reading_list.db`) - Lista de Leitura

| Campo | Descrição |
|-------|-----------|
| `id` | ID do artigo (referência ao SQLite) |
| `content` | Conteúdo HTML completo do artigo |
| `content_type` | Tipo: "html" ou "text" |
| `imported_at` | Data/hora da importação |

**Características:**
- Banco NoSQL key-value (alta performance)
- Armazena **conteúdo completo** para leitura offline
- Scraper inteligente com suporte a:
  - Medium (via proxies Freedium/Scribe.rip)
  - Dev.to
  - GitHub
  - Substack
  - Sites genéricos

### Fluxo de Dados no Dashboard

| Estatística | Fonte | Descrição |
|-------------|-------|-----------|
| **Total de Artigos Extraídos** | SQLite | Links encontrados nas varreduras |
| **Artigos Salvos Localmente** | BBolt | Artigos importados para leitura offline |

---

## 🔧 Solução de Problemas

### Erro de autenticação
- Verifique se está usando **Senha de App** (não a senha normal)
- Confirme que o IMAP está habilitado no Gmail
- Verifique se a Verificação em 2 etapas está ativada

### Porta 8080 em uso
```bash
# Verificar o que está usando a porta
netstat -ano | findstr :8080

# Parar containers antigos
docker stop $(docker ps -q)
```

### Container não inicia
```bash
# Ver logs detalhados
docker-compose logs gmail-scanner

# Rebuild completo
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

### Erro de localStorage no navegador
- Limpe o cache: F12 → Application → Local Storage → Clear
- Ou teste em aba anônima

---

## 🛠️ Tecnologias

| Componente | Tecnologia |
|------------|------------|
| Backend | Go 1.24, Gorilla Mux |
| Frontend | React 18, Vite, Tailwind CSS |
| Banco SQL | SQLite (modernc.org/sqlite) |
| Banco NoSQL | BBolt |
| IMAP | emersion/go-imap |
| Auth | JWT (golang-jwt) |
| Container | Docker, Alpine Linux |

---

## 📄 Licença

MIT License - veja [LICENSE](LICENSE) para detalhes.

---

## 🤝 Contribuições

1. Fork o repositório
2. Crie uma branch: `git checkout -b feature/nova-feature`
3. Commit: `git commit -m 'Adiciona nova feature'`
4. Push: `git push origin feature/nova-feature`
5. Abra um Pull Request

---

**Versão:** 0.5.0  
**Última atualização:** 28 de novembro de 2025
