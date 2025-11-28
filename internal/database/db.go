package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Article representa um artigo/link extraído de uma newsletter
type Article struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	Newsletter  string `json:"newsletter"` // Nome da newsletter (From do email)
	EmailDate   string `json:"email_date"` // Data do email
	Folder      string `json:"folder"`     // Pasta IMAP de origem
	CreatedAt   string `json:"created_at"`
}

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	sqlDb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := sqlDb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &Database{db: sqlDb}

	// Create table
	if err := db.CreateTable(); err != nil {
		return nil, err
	}

	return db, nil
}

func (d *Database) CreateTable() error {
	// Tabela única de artigos
	query := `
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		title TEXT,
		description TEXT,
		domain TEXT,
		newsletter TEXT,
		email_date TEXT,
		folder TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	)
	`

	_, err := d.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create articles table: %w", err)
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_articles_domain ON articles(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_newsletter ON articles(newsletter)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_email_date ON articles(email_date)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_url ON articles(url)`,
	}

	for _, idx := range indexes {
		if _, err := d.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// IndexArticle salva um artigo no banco (ignora se URL já existe)
func (d *Database) IndexArticle(article *Article) error {
	query := `
	INSERT OR IGNORE INTO articles (url, title, description, domain, newsletter, email_date, folder, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`

	_, err := d.db.Exec(query, article.URL, article.Title, article.Description, article.Domain, article.Newsletter, article.EmailDate, article.Folder)
	if err != nil {
		return fmt.Errorf("failed to index article: %w", err)
	}

	return nil
}

// IndexArticles salva múltiplos artigos
func (d *Database) IndexArticles(articles []Article) error {
	for _, article := range articles {
		if err := d.IndexArticle(&article); err != nil {
			return err
		}
	}
	return nil
}

// GetAllArticles retorna todos os artigos com paginação e filtros
func (d *Database) GetAllArticles(page, pageSize int, domain, search, newsletter string) ([]Article, int, error) {
	offset := (page - 1) * pageSize

	countQuery := `SELECT COUNT(*) FROM articles WHERE 1=1`
	selectQuery := `
	SELECT id, url, title, description, domain, newsletter, email_date, folder, created_at
	FROM articles
	WHERE 1=1
	`

	args := []interface{}{}
	countArgs := []interface{}{}

	// Filtro de domínio
	if domain != "" {
		countQuery += " AND domain = ?"
		selectQuery += " AND domain = ?"
		args = append(args, domain)
		countArgs = append(countArgs, domain)
	}

	// Filtro de newsletter
	if newsletter != "" {
		countQuery += " AND newsletter LIKE ?"
		selectQuery += " AND newsletter LIKE ?"
		searchNewsletter := "%" + newsletter + "%"
		args = append(args, searchNewsletter)
		countArgs = append(countArgs, searchNewsletter)
	}

	// Filtro de busca
	if search != "" {
		searchTerm := "%" + search + "%"
		countQuery += " AND (title LIKE ? OR description LIKE ? OR url LIKE ? OR newsletter LIKE ?)"
		selectQuery += " AND (title LIKE ? OR description LIKE ? OR url LIKE ? OR newsletter LIKE ?)"
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
		countArgs = append(countArgs, searchTerm, searchTerm, searchTerm, searchTerm)
	}

	// Contar total
	var total int
	err := d.db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count articles: %w", err)
	}

	// Buscar resultados paginados
	selectQuery += " ORDER BY email_date DESC, created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := d.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get articles: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var article Article
		var emailDate, createdAt sql.NullString
		err := rows.Scan(&article.ID, &article.URL, &article.Title, &article.Description,
			&article.Domain, &article.Newsletter, &emailDate, &article.Folder, &createdAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan article: %w", err)
		}
		if emailDate.Valid {
			article.EmailDate = emailDate.String
		}
		if createdAt.Valid {
			article.CreatedAt = createdAt.String
		}
		articles = append(articles, article)
	}

	return articles, total, nil
}

// GetStats retorna estatísticas gerais
func (d *Database) GetStats() (map[string]interface{}, error) {
	var totalArticles int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM articles`).Scan(&totalArticles)
	if err != nil {
		// Se a tabela não existe ainda, retornar 0
		totalArticles = 0
	}

	stats := map[string]interface{}{
		"total_emails": totalArticles, // Mantém nome para compatibilidade com frontend
	}

	return stats, nil
}

// GetArticleStats retorna estatísticas sobre os artigos
func (d *Database) GetArticleStats() (map[string]interface{}, error) {
	// Total de artigos
	var totalArticles int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM articles`).Scan(&totalArticles)
	if err != nil {
		return nil, fmt.Errorf("failed to count articles: %w", err)
	}

	// Artigos por domínio (top 10)
	domainQuery := `
	SELECT domain, COUNT(*) as count
	FROM articles
	WHERE domain != ''
	GROUP BY domain
	ORDER BY count DESC
	LIMIT 10
	`

	rows, err := d.db.Query(domainQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain stats: %w", err)
	}
	defer rows.Close()

	byDomain := make(map[string]int)
	for rows.Next() {
		var domain string
		var count int
		if err := rows.Scan(&domain, &count); err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}
		byDomain[domain] = count
	}

	// Newsletters únicas
	var totalNewsletters int
	d.db.QueryRow(`SELECT COUNT(DISTINCT newsletter) FROM articles`).Scan(&totalNewsletters)

	stats := map[string]interface{}{
		"total_links":       totalArticles, // Compatibilidade com frontend
		"total_newsletters": totalNewsletters,
		"by_domain":         byDomain,
	}

	return stats, nil
}

// DeleteArticle deleta um artigo pelo ID
func (d *Database) DeleteArticle(articleID int64) error {
	query := `DELETE FROM articles WHERE id = ?`
	result, err := d.db.Exec(query, articleID)
	if err != nil {
		return fmt.Errorf("failed to delete article: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("article not found")
	}
	return nil
}

// GetNewsletters retorna lista de newsletters únicas
func (d *Database) GetNewsletters() ([]string, error) {
	query := `
	SELECT DISTINCT newsletter 
	FROM articles 
	WHERE newsletter != '' 
	ORDER BY newsletter
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get newsletters: %w", err)
	}
	defer rows.Close()

	var newsletters []string
	for rows.Next() {
		var newsletter string
		if err := rows.Scan(&newsletter); err != nil {
			return nil, fmt.Errorf("failed to scan newsletter: %w", err)
		}
		newsletters = append(newsletters, newsletter)
	}

	return newsletters, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
