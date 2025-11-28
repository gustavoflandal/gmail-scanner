package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gustavoflandal/gmail-scanner/internal/auth"
	"github.com/gustavoflandal/gmail-scanner/internal/database"
	"github.com/gustavoflandal/gmail-scanner/internal/nosql"
	"github.com/gustavoflandal/gmail-scanner/internal/scraper"
	"github.com/sirupsen/logrus"
)

var (
	log          *logrus.Logger
	db           *database.Database
	nosqlDB      *nosql.NoSQLDB
	scanMutex    sync.Mutex
	scanStatus   *ScanStatus
	isScanning   bool
	cancelScan   chan bool
	scanProgress *ScanProgress
)

// ScanStatus representa o estado da varredura
type ScanStatus struct {
	IsRunning         bool      `json:"is_running"`
	LastScanTime      time.Time `json:"last_scan_time,omitempty"`
	LastEmailsScanned int       `json:"last_emails_scanned"`
	LastError         string    `json:"last_error,omitempty"`
}

// ScanProgress representa o progresso detalhado da varredura
type ScanProgress struct {
	CurrentFolder    string `json:"current_folder"`
	FoldersTotal     int    `json:"folders_total"`
	FoldersProcessed int    `json:"folders_processed"`
	EmailsTotal      int    `json:"emails_total"`
	EmailsProcessed  int    `json:"emails_processed"`
	ArticlesFound    int    `json:"articles_found"`
	PercentComplete  int    `json:"percent_complete"`
	Status           string `json:"status"`
}

// ScanRequest representa os parâmetros de varredura
type ScanRequest struct {
	Folders []string `json:"folders"`
}

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	scanStatus = &ScanStatus{
		IsRunning: false,
	}

	scanProgress = &ScanProgress{
		Status: "idle",
	}

	cancelScan = make(chan bool, 1)
}

func main() {
	// Create data directory if needed
	if _, err := os.Stat("./data"); os.IsNotExist(err) {
		os.Mkdir("./data", 0755)
	}

	// Inicializar autenticação simples
	jwtSecret := os.Getenv("JWT_SECRET")
	auth.Init(jwtSecret)

	var err error
	db, err = database.NewDatabase("./data/emails.db")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	// Inicializar banco NoSQL (BBolt)
	nosqlDB, err = nosql.NewNoSQLDB("./data/reading_list.db")
	if err != nil {
		log.Fatalf("failed to initialize nosql database: %v", err)
	}
	defer nosqlDB.Close()

	router := mux.NewRouter()
	router.Use(corsMiddleware)

	// Auth routes (públicas)
	router.HandleFunc("/api/auth/login", handleLogin).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/auth/logout", handleLogout).Methods("POST", "OPTIONS")

	// API routes (requerem autenticação)
	router.HandleFunc("/api/articles", authMiddleware(getAllArticles)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/articles/{id}", authMiddleware(deleteArticle)).Methods("DELETE", "OPTIONS")
	router.HandleFunc("/api/articles/stats", authMiddleware(getArticleStats)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/newsletters", authMiddleware(getNewsletters)).Methods("GET", "OPTIONS")

	// Rotas legadas para compatibilidade com frontend
	router.HandleFunc("/api/links", authMiddleware(getAllArticles)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/links/{id}", authMiddleware(deleteArticle)).Methods("DELETE", "OPTIONS")
	router.HandleFunc("/api/links/stats", authMiddleware(getArticleStats)).Methods("GET", "OPTIONS")

	// Rotas NoSQL - Lista de Leitura (rotas específicas ANTES das rotas com parâmetros)
	router.HandleFunc("/api/reading-list/import", authMiddleware(importToReadingList)).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/reading-list/imported-ids", authMiddleware(getImportedIDs)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/reading-list", authMiddleware(getAllFromReadingList)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/reading-list/{id}", authMiddleware(getFromReadingList)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/reading-list/{id}", authMiddleware(deleteFromReadingList)).Methods("DELETE", "OPTIONS")

	router.HandleFunc("/api/scan", authMiddleware(startScan)).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/scan-status", authMiddleware(getScanStatus)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/scan-progress", authMiddleware(getScanProgress)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/scan-cancel", authMiddleware(cancelScanHandler)).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/folders", authMiddleware(getFolders)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/stats", authMiddleware(getStats)).Methods("GET", "OPTIONS")

	// API routes públicas
	router.HandleFunc("/api/health", getHealth).Methods("GET", "OPTIONS")

	// SPA handler - serve index.html para rotas do React Router
	router.PathPrefix("/").HandlerFunc(spaHandler)

	// Cleanup de sessões expiradas a cada hora
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			auth.CleanupExpiredSessions()
		}
	}()

	port := ":8080"
	log.Infof("Server listening on %s", port)
	log.Infof("Login endpoint: POST /api/auth/login")
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler serve arquivos estáticos ou index.html para rotas SPA
func spaHandler(w http.ResponseWriter, r *http.Request) {
	// Caminho do diretório público
	publicDir := "./web/public"
	
	// Caminho do arquivo solicitado
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	
	// Verificar se o arquivo existe
	filePath := publicDir + path
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Se não existe, servir index.html (SPA fallback)
		http.ServeFile(w, r, publicDir+"/index.html")
		return
	}
	
	// Se existe, servir o arquivo
	http.ServeFile(w, r, filePath)
}

// authMiddleware verifica autenticação
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetAuthToken(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "não autorizado"})
			return
		}

		session, err := auth.ValidateToken(token)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "token inválido"})
			return
		}

		// Adicionar email ao contexto (opcional)
		_ = session.Email

		next.ServeHTTP(w, r)
	}
}

func getHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getStats(w http.ResponseWriter, r *http.Request) {
	// Stats do banco SQLite (artigos extraídos)
	dbStats, err := db.GetStats()
	if err != nil {
		log.Errorf("stats error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "stats failed"})
		return
	}

	// Stats do banco NoSQL (artigos importados/salvos localmente)
	nosqlStats, err := nosqlDB.GetStats()
	if err != nil {
		log.Warnf("nosql stats error: %v", err)
		nosqlStats = map[string]interface{}{"total_imported": 0}
	}

	// Combinar stats
	stats := map[string]interface{}{
		"total_articles":  dbStats["total_emails"],  // Total de artigos extraídos
		"total_imported":  nosqlStats["total_imported"], // Total de artigos salvos localmente
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// startScan inicia uma varredura manual de emails
func startScan(w http.ResponseWriter, r *http.Request) {
	scanMutex.Lock()
	if isScanning {
		scanMutex.Unlock()
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "varredura já em andamento"})
		return
	}
	isScanning = true
	scanStatus.IsRunning = true
	scanMutex.Unlock()

	// Parse request body para obter pastas selecionadas
	var scanReq ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&scanReq); err != nil {
		// Se não houver body, usar pastas padrão
		scanReq.Folders = []string{"INBOX"}
	}

	// Se nenhuma pasta foi especificada, usar INBOX
	if len(scanReq.Folders) == 0 {
		scanReq.Folders = []string{"INBOX"}
	}

	// Obter token e sessão
	token, err := auth.GetAuthToken(r)
	if err != nil {
		scanMutex.Lock()
		isScanning = false
		scanStatus.IsRunning = false
		scanMutex.Unlock()

		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "não autorizado"})
		return
	}

	session, err := auth.GetSession(token)
	if err != nil {
		scanMutex.Lock()
		isScanning = false
		scanStatus.IsRunning = false
		scanMutex.Unlock()

		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "sessão inválida"})
		return
	}

	// Limpar canal de cancelamento
	select {
	case <-cancelScan:
	default:
	}

	// Responder imediatamente
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "started",
		"message": "varredura iniciada",
		"folders": scanReq.Folders,
	})

	// Executar varredura em goroutine
	go performScan(session, scanReq.Folders)
}

// performScan executa a varredura de emails
func performScan(session *auth.Session, folders []string) {
	defer func() {
		scanMutex.Lock()
		isScanning = false
		scanStatus.IsRunning = false
		scanStatus.LastScanTime = time.Now()
		scanProgress.Status = "completed"
		scanMutex.Unlock()
	}()

	log.Infof("Starting email scan for %s in folders: %v", session.Email, folders)

	// Atualizar progresso inicial
	scanMutex.Lock()
	scanProgress.CurrentFolder = ""
	scanProgress.FoldersTotal = len(folders)
	scanProgress.FoldersProcessed = 0
	scanProgress.EmailsTotal = 0
	scanProgress.EmailsProcessed = 0
	scanProgress.ArticlesFound = 0
	scanProgress.PercentComplete = 0
	scanProgress.Status = "connecting"
	scanMutex.Unlock()

	// Conectar IMAP
	imapClient, err := session.GetIMAPClient()
	if err != nil {
		scanMutex.Lock()
		scanStatus.LastError = fmt.Sprintf("Falha ao conectar IMAP: %v", err)
		scanProgress.Status = "error"
		scanMutex.Unlock()
		log.Errorf("IMAP connection failed: %v", err)
		return
	}
	defer imapClient.Close()

	scanMutex.Lock()
	scanProgress.Status = "scanning"
	scanMutex.Unlock()

	totalArticleCount := 0

	// Processar cada pasta
	for i, folder := range folders {
		// Verificar se foi cancelado
		select {
		case <-cancelScan:
			log.Infof("Scan cancelled by user")
			scanMutex.Lock()
			scanStatus.LastError = "Varredura cancelada pelo usuário"
			scanProgress.Status = "cancelled"
			scanMutex.Unlock()
			return
		default:
		}

		scanMutex.Lock()
		scanProgress.CurrentFolder = folder
		scanProgress.FoldersProcessed = i
		scanProgress.PercentComplete = (i * 100) / len(folders)
		scanMutex.Unlock()

		log.Infof("Scanning folder: %s (%d/%d)", folder, i+1, len(folders))

		// Buscar TODAS as mensagens da pasta (limit = 0)
		messages, err := imapClient.FetchMessages(folder, 0)
		if err != nil {
			log.Warnf("Failed to fetch messages from %s: %v", folder, err)
			continue
		}

		log.Infof("Fetched %d messages from folder %s", len(messages), folder)

		scanMutex.Lock()
		scanProgress.EmailsTotal += len(messages)
		scanMutex.Unlock()

		// Processar cada mensagem e salvar artigos
		for j, msg := range messages {
			// Verificar cancelamento a cada 10 emails
			if j%10 == 0 {
				select {
				case <-cancelScan:
					log.Infof("Scan cancelled by user")
					scanMutex.Lock()
					scanStatus.LastError = "Varredura cancelada pelo usuário"
					scanProgress.Status = "cancelled"
					scanMutex.Unlock()
					return
				default:
				}
			}

			// Salvar cada link como um artigo
			for _, link := range msg.Links {
				article := &database.Article{
					URL:         link.URL,
					Title:       link.Title,
					Description: link.Description,
					Domain:      link.Domain,
					Newsletter:  msg.From,
					EmailDate:   msg.Date.Format(time.RFC3339),
					Folder:      msg.Folder,
				}

				if err := db.IndexArticle(article); err != nil {
					log.Warnf("Failed to index article: %v", err)
					continue
				}

				totalArticleCount++
			}

			scanMutex.Lock()
			scanProgress.EmailsProcessed++
			scanProgress.ArticlesFound = totalArticleCount
			scanMutex.Unlock()

			// Log a cada 50 emails processados
			if (j+1)%50 == 0 {
				log.Infof("Processed %d emails, found %d articles so far...", j+1, totalArticleCount)
			}
		}
	}

	scanMutex.Lock()
	scanStatus.LastEmailsScanned = scanProgress.EmailsProcessed
	scanStatus.LastError = ""
	scanProgress.FoldersProcessed = len(folders)
	scanProgress.PercentComplete = 100
	scanProgress.ArticlesFound = totalArticleCount
	scanProgress.Status = "completed"
	scanMutex.Unlock()

	log.Infof("Scan completed: %d articles extracted from %d emails in %d folders",
		totalArticleCount, scanProgress.EmailsProcessed, len(folders))
}

// getScanStatus retorna o status da varredura
func getScanStatus(w http.ResponseWriter, r *http.Request) {
	scanMutex.Lock()
	status := *scanStatus
	scanMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleLogin processa login com email e senha
func handleLogin(w http.ResponseWriter, r *http.Request) {
	var loginReq auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "requisição inválida"})
		return
	}

	if loginReq.Email == "" || loginReq.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "email e senha são obrigatórios"})
		return
	}

	log.Infof("Login attempt for %s", loginReq.Email)

	response, err := auth.Authenticate(loginReq.Email, loginReq.Password)
	if err != nil {
		log.Errorf("Authentication failed for %s: %v", loginReq.Email, err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Definir cookie
	auth.SetAuthCookie(w, response.Token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Infof("User authenticated successfully: %s", loginReq.Email)
}

// handleLogout faz logout do usuário
func handleLogout(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetAuthToken(r)
	if err == nil {
		auth.Logout(token)
	}

	auth.ClearAuthCookie(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
	log.Info("User logged out")
}

// getScanProgress retorna o progresso detalhado da varredura
func getScanProgress(w http.ResponseWriter, r *http.Request) {
	scanMutex.Lock()
	progress := *scanProgress
	scanMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

// cancelScanHandler cancela a varredura em andamento
func cancelScanHandler(w http.ResponseWriter, r *http.Request) {
	scanMutex.Lock()
	if !isScanning {
		scanMutex.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "nenhuma varredura em andamento"})
		return
	}
	scanMutex.Unlock()

	// Enviar sinal de cancelamento
	select {
	case cancelScan <- true:
		log.Info("Scan cancellation requested")
	default:
		// Canal já tem sinal pendente
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "cancelling",
		"message": "cancelamento solicitado",
	})
}

// getFolders retorna lista de pastas IMAP disponíveis
func getFolders(w http.ResponseWriter, r *http.Request) {
	// Obter token e sessão
	token, err := auth.GetAuthToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "não autorizado"})
		return
	}

	session, err := auth.GetSession(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "sessão inválida"})
		return
	}

	// Conectar IMAP
	imapClient, err := session.GetIMAPClient()
	if err != nil {
		log.Errorf("IMAP connection failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao conectar IMAP"})
		return
	}
	defer imapClient.Close()

	// Listar pastas
	folders, err := imapClient.ListFolders()
	if err != nil {
		log.Errorf("Failed to list folders: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao listar pastas"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"folders": folders,
	})
}

// getAllArticles retorna todos os artigos com paginação e filtros
func getAllArticles(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSizeStr := r.URL.Query().Get("page_size")
	pageSize := 50000 // Aumentado para carregar todos os artigos
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	log.Infof("GetAllArticles: page=%d, pageSize=%d (requested: %s)", page, pageSize, pageSizeStr)

	domain := r.URL.Query().Get("domain")
	search := r.URL.Query().Get("q")
	newsletter := r.URL.Query().Get("newsletter")

	articles, total, err := db.GetAllArticles(page, pageSize, domain, search, newsletter)
	if err != nil {
		log.Errorf("Failed to get articles: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao buscar artigos"})
		return
	}

	// Converter para formato compatível com frontend (usando "links" como chave)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"links":     articles, // Mantém compatibilidade com frontend
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// deleteArticle deleta um artigo específico
func deleteArticle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warnf("Invalid article ID: %s", idStr)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "ID inválido"})
		return
	}

	log.Infof("Attempting to delete article with ID: %d", id)

	if err := db.DeleteArticle(id); err != nil {
		log.Errorf("Failed to delete article %d: %v", id, err)
		if err.Error() == "article not found" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "artigo não encontrado"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao deletar artigo"})
		return
	}

	log.Infof("Successfully deleted article with ID: %d", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "artigo deletado com sucesso"})
}

// getArticleStats retorna estatísticas sobre os artigos
func getArticleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetArticleStats()
	if err != nil {
		log.Errorf("Failed to get article stats: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao buscar estatísticas"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// getNewsletters retorna lista de newsletters únicas
func getNewsletters(w http.ResponseWriter, r *http.Request) {
	newsletters, err := db.GetNewsletters()
	if err != nil {
		log.Errorf("Failed to get newsletters: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao buscar newsletters"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"newsletters": newsletters,
	})
}

// ==================== NoSQL Reading List Handlers ====================

// ImportRequest representa a requisição de importação
type ImportRequest struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	Newsletter  string `json:"newsletter"`
	EmailDate   string `json:"email_date"`
	Folder      string `json:"folder"`
}

// importToReadingList importa um artigo para a lista de leitura (NoSQL)
func importToReadingList(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("Failed to decode import request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "dados inválidos"})
		return
	}

	// Buscar conteúdo do artigo
	log.Infof("Fetching article content from: %s", req.URL)
	articleContent, err := scraper.FetchArticleContent(req.URL)

	var content string
	var contentType string

	if err != nil {
		log.Warnf("Failed to fetch article content: %v - saving without content", err)
		content = ""
		contentType = ""
	} else {
		content = articleContent.Content
		contentType = articleContent.ContentType
		log.Infof("Successfully fetched article content (%d chars)", len(content))
	}

	article := nosql.Article{
		ID:          req.ID,
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
		Domain:      req.Domain,
		Newsletter:  req.Newsletter,
		EmailDate:   req.EmailDate,
		Folder:      req.Folder,
		Content:     content,
		ContentType: contentType,
	}

	if err := nosqlDB.ImportArticle(article); err != nil {
		log.Errorf("Failed to import article: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao importar artigo"})
		return
	}

	log.Infof("Article imported to reading list: ID=%d, Title=%s, ContentSize=%d", req.ID, req.Title, len(content))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "artigo importado com sucesso",
		"id":           req.ID,
		"has_content":  content != "",
		"content_size": len(content),
	})
}

// getFromReadingList obtém um artigo da lista de leitura
func getFromReadingList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "ID inválido"})
		return
	}

	article, err := nosqlDB.GetArticle(id)
	if err != nil {
		log.Errorf("Failed to get article from reading list: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao buscar artigo"})
		return
	}

	if article == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "artigo não encontrado na lista de leitura"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(article)
}

// deleteFromReadingList remove um artigo da lista de leitura
func deleteFromReadingList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "ID inválido"})
		return
	}

	if err := nosqlDB.DeleteArticle(id); err != nil {
		log.Errorf("Failed to delete article from reading list: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao remover artigo"})
		return
	}

	log.Infof("Article removed from reading list: ID=%d", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "artigo removido da lista de leitura"})
}

// getAllFromReadingList obtém todos os artigos da lista de leitura
func getAllFromReadingList(w http.ResponseWriter, r *http.Request) {
	articles, err := nosqlDB.GetAllImported()
	if err != nil {
		log.Errorf("Failed to get reading list: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao buscar lista de leitura"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"articles": articles,
		"total":    len(articles),
	})
}

// getImportedIDs retorna os IDs de todos os artigos importados
func getImportedIDs(w http.ResponseWriter, r *http.Request) {
	ids, err := nosqlDB.GetImportedIDs()
	if err != nil {
		log.Errorf("Failed to get imported IDs: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "falha ao buscar IDs importados"})
		return
	}

	log.Infof("Returning %d imported IDs", len(ids))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"imported_ids": ids,
	})
}
