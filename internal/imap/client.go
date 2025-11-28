package imap

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

// Client representa um cliente IMAP conectado
type Client struct {
	conn  *client.Client
	email string
}

// Message representa uma mensagem de email
type Message struct {
	MessageID      string
	From           string
	Subject        string
	Date           time.Time
	Body           string
	SnippetPreview string
	Folder         string
	IsRead         bool
	Links          []EmailLink
}

// EmailLink representa um link extraído do corpo do email
type EmailLink struct {
	URL         string
	Title       string
	Description string
	Domain      string
	Position    int
}

// Connect estabelece conexão com servidor IMAP do Gmail
func Connect(email, password string) (*Client, error) {
	log.Infof("Connecting to IMAP server for %s", email)

	// Conectar ao Gmail IMAP (SSL/TLS)
	conn, err := client.DialTLS("imap.gmail.com:993", &tls.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}

	// Autenticar
	if err := conn.Login(email, password); err != nil {
		conn.Logout()
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	log.Infof("Successfully authenticated as %s", email)

	return &Client{
		conn:  conn,
		email: email,
	}, nil
}

// Close fecha a conexão IMAP
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Logout()
	}
	return nil
}

// ListFolders retorna lista de todas as pastas/labels
func (c *Client) ListFolders() ([]string, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.conn.List("", "*", mailboxes)
	}()

	var folders []string
	for m := range mailboxes {
		folders = append(folders, m.Name)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	log.Infof("Found %d folders", len(folders))
	return folders, nil
}

// FetchMessages busca mensagens de uma pasta específica
// Se limit = 0, busca TODAS as mensagens
func (c *Client) FetchMessages(folder string, limit uint32) ([]*Message, error) {
	// Selecionar pasta
	mbox, err := c.conn.Select(folder, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	if mbox.Messages == 0 {
		log.Infof("No messages in folder %s", folder)
		return []*Message{}, nil
	}

	// Determinar range de mensagens
	from := uint32(1)
	to := mbox.Messages

	// Se limit > 0, buscar apenas as últimas 'limit' mensagens
	if limit > 0 && mbox.Messages > limit {
		from = mbox.Messages - limit + 1
	}

	log.Infof("Fetching messages %d:%d from folder %s (total: %d)", from, to, folder, mbox.Messages)

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	// Buscar envelope, flags e corpo completo usando BODY[]
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{
		imap.FetchEnvelope,
		imap.FetchFlags,
		imap.FetchUid,
		section.FetchItem(),
	}

	go func() {
		done <- c.conn.Fetch(seqset, items, messages)
	}()

	var result []*Message
	for msg := range messages {
		if msg == nil || msg.Envelope == nil {
			continue
		}

		// Construir mensagem
		message := &Message{
			MessageID: msg.Envelope.MessageId,
			Subject:   msg.Envelope.Subject,
			Date:      msg.Envelope.Date,
			Folder:    folder,
			IsRead:    false,
		}

		// Verificar se está lida
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				message.IsRead = true
				break
			}
		}

		// Extrair remetente
		if len(msg.Envelope.From) > 0 {
			from := msg.Envelope.From[0]
			if from.PersonalName != "" {
				message.From = fmt.Sprintf("%s <%s@%s>", from.PersonalName, from.MailboxName, from.HostName)
			} else {
				message.From = fmt.Sprintf("%s@%s", from.MailboxName, from.HostName)
			}
		}

		// Usar subject como snippet por enquanto (mais rápido)
		message.SnippetPreview = msg.Envelope.Subject
		if len(message.SnippetPreview) > 200 {
			message.SnippetPreview = message.SnippetPreview[:200] + "..."
		}

		// Extrair corpo do email usando GetBody com a seção correta
		bodyReader := msg.GetBody(section)
		if bodyReader != nil {
			body, err := io.ReadAll(bodyReader)
			if err != nil {
				log.Warnf("Failed to read body: %v", err)
			} else if len(body) > 0 {
				log.Infof("Got body with %d bytes for: %s", len(body), message.Subject)

				// Tentar extrair HTML do corpo MIME
				htmlContent := extractHTMLFromMIME(body)
				if htmlContent != "" {
					message.Body = htmlContent

					// Extrair links do corpo HTML
					message.Links = extractLinks(htmlContent)
					if len(message.Links) > 0 {
						log.Infof("Extracted %d links from email: %s", len(message.Links), message.Subject)
					} else {
						log.Infof("No links found in email: %s", message.Subject)
					}
				} else {
					// Fallback: usar corpo bruto
					message.Body = string(body)
					message.Links = extractLinks(string(body))
					if len(message.Links) > 0 {
						log.Infof("Extracted %d links (raw) from email: %s", len(message.Links), message.Subject)
					}
				}
			} else {
				log.Warnf("Empty body for email: %s", message.Subject)
			}
		} else {
			log.Warnf("No body reader for email: %s (body sections: %d)", message.Subject, len(msg.Body))
		}

		result = append(result, message)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	log.Infof("Fetched %d messages from folder %s", len(result), folder)
	return result, nil
}

// fetchSnippet busca um preview do corpo da mensagem
func (c *Client) fetchSnippet(uid uint32, folder string) (string, error) {
	// Re-selecionar pasta se necessário
	c.conn.Select(folder, true)

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- c.conn.UidFetch(seqset, items, messages)
	}()

	msg := <-messages
	if msg == nil {
		return "", fmt.Errorf("no message found")
	}

	r := msg.GetBody(section)
	if r == nil {
		return "", fmt.Errorf("no body found")
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	// Extrair texto simples (primeiros 200 caracteres)
	text := string(body)

	// Tentar encontrar conteúdo text/plain
	if strings.Contains(text, "Content-Type: text/plain") {
		parts := strings.Split(text, "\n\n")
		if len(parts) > 1 {
			text = parts[1]
		}
	}

	// Limpar e truncar
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\n", " ")

	if len(text) > 200 {
		text = text[:200] + "..."
	}

	return text, nil
}

// FetchAllMessages busca mensagens de todas as pastas importantes
func (c *Client) FetchAllMessages(limit uint32) ([]*Message, error) {
	// Pastas principais do Gmail
	folders := []string{
		"INBOX",
		"[Gmail]/Sent Mail",
		"[Gmail]/Important",
		"[Gmail]/Starred",
	}

	var allMessages []*Message

	for _, folder := range folders {
		messages, err := c.FetchMessages(folder, limit)
		if err != nil {
			log.Warnf("Failed to fetch from folder %s: %v", folder, err)
			continue
		}
		allMessages = append(allMessages, messages...)
	}

	log.Infof("Fetched total of %d messages from all folders", len(allMessages))
	return allMessages, nil
}

// TestConnection testa se as credenciais são válidas
func TestConnection(email, password string) error {
	client, err := Connect(email, password)
	if err != nil {
		return err
	}
	defer client.Close()

	log.Info("Connection test successful")
	return nil
}

// extractHTMLFromMIME extrai o conteúdo HTML de um corpo MIME
func extractHTMLFromMIME(rawBody []byte) string {
	// Tentar parsear como mensagem MIME
	r := bytes.NewReader(rawBody)

	// Primeiro tentar como email completo
	mr, err := mail.CreateReader(r)
	if err != nil {
		// Se falhar, tentar como entidade única
		r.Reset(rawBody)
		entity, err := message.Read(r)
		if err != nil {
			// Retornar corpo bruto se não conseguir parsear
			return string(rawBody)
		}

		// Se for multipart, iterar pelas partes
		mpReader := entity.MultipartReader()
		if mpReader != nil {
			for {
				part, err := mpReader.NextPart()
				if err != nil {
					break
				}

				contentType, _, _ := part.Header.ContentType()
				if strings.Contains(contentType, "text/html") {
					body, err := io.ReadAll(part.Body)
					if err == nil && len(body) > 0 {
						return string(body)
					}
				}
			}
		}

		// Se não for multipart, verificar se é HTML
		contentType, _, _ := entity.Header.ContentType()
		if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "text/plain") {
			body, err := io.ReadAll(entity.Body)
			if err == nil {
				return string(body)
			}
		}

		return string(rawBody)
	}
	defer mr.Close()

	var htmlContent string
	var textContent string

	// Iterar pelas partes do email
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			body, err := io.ReadAll(part.Body)
			if err != nil {
				continue
			}

			if strings.Contains(contentType, "text/html") {
				htmlContent = string(body)
			} else if strings.Contains(contentType, "text/plain") && textContent == "" {
				textContent = string(body)
			}
		}
	}

	// Preferir HTML sobre texto plano
	if htmlContent != "" {
		return htmlContent
	}
	if textContent != "" {
		return textContent
	}

	return string(rawBody)
}

// extractHTMLBody extrai o corpo HTML do email
func (c *Client) extractHTMLBody(msg *imap.Message) string {
	if msg.BodyStructure == nil {
		return ""
	}

	// Procurar por parte HTML
	var section *imap.BodySectionName
	var findHTML func(*imap.BodyStructure) *imap.BodySectionName

	findHTML = func(bs *imap.BodyStructure) *imap.BodySectionName {
		if bs.MIMEType == "text" && bs.MIMESubType == "html" {
			return &imap.BodySectionName{}
		}

		for i, part := range bs.Parts {
			if part.MIMEType == "text" && part.MIMESubType == "html" {
				// Usar path ao invés de Specifier
				section := &imap.BodySectionName{}
				section.Path = []int{i + 1}
				return section
			}
			if result := findHTML(part); result != nil {
				return result
			}
		}
		return nil
	}

	section = findHTML(msg.BodyStructure)
	if section == nil {
		return ""
	}

	// Obter corpo
	r := msg.GetBody(section)
	if r == nil {
		return ""
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return ""
	}

	return string(body)
}

// extractLinks extrai links relevantes do corpo HTML
func extractLinks(htmlBody string) []EmailLink {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
	if err != nil {
		return []EmailLink{}
	}

	var links []EmailLink
	position := 0
	seenURLs := make(map[string]bool)

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		// Ignorar links irrelevantes
		if isIgnorableLink(href) {
			return
		}

		// Parse URL
		parsedURL, err := url.Parse(href)
		if err != nil {
			return
		}

		// Apenas links HTTP/HTTPS
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return
		}

		// Normalizar URL (remover parâmetros de tracking)
		normalizedURL := normalizeURL(parsedURL)

		// Evitar duplicatas usando URL normalizada
		if seenURLs[normalizedURL] {
			return
		}
		seenURLs[normalizedURL] = true

		// Extrair título usando múltiplas estratégias
		title := extractTitleFromLink(s, href, parsedURL)

		// Validar título: deve ter pelo menos 20 caracteres e iniciar com letra maiúscula
		if !isValidTitle(title) {
			return
		}

		// Limitar tamanho do título
		if len(title) > 200 {
			title = title[:200] + "..."
		}

		// Tentar pegar descrição (próximo elemento <p>)
		description := ""
		parent := s.Parent()
		next := parent.Next()
		if next.Is("p") {
			description = strings.TrimSpace(next.Text())
			if len(description) > 300 {
				description = description[:300] + "..."
			}
		}

		links = append(links, EmailLink{
			URL:         normalizedURL, // Usar URL normalizada
			Title:       title,
			Description: description,
			Domain:      parsedURL.Hostname(),
			Position:    position,
		})

		position++
	})

	log.Infof("Extracted %d links from email", len(links))
	return links
}

// normalizeURL remove parâmetros de tracking e normaliza a URL
func normalizeURL(u *url.URL) string {
	// Parâmetros de tracking a serem removidos
	trackingParams := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"ref", "source", "mc_cid", "mc_eid",
		"fbclid", "gclid", "dclid",
		"_ga", "_gl",
		"oly_enc_id", "oly_anon_id",
		"vero_id", "vero_conv",
		"spm", "share_token",
		"si", "feature",
	}

	// Copiar URL
	normalized := *u

	// Remover parâmetros de tracking
	query := normalized.Query()
	for _, param := range trackingParams {
		query.Del(param)
	}

	// Se não sobrou nenhum parâmetro, remover o "?"
	if len(query) == 0 {
		normalized.RawQuery = ""
	} else {
		normalized.RawQuery = query.Encode()
	}

	// Remover fragmento (#)
	normalized.Fragment = ""

	// Remover barra final desnecessária
	normalized.Path = strings.TrimSuffix(normalized.Path, "/")

	return normalized.String()
}

// isValidTitle verifica se o título é válido:
// - Deve ter pelo menos 20 caracteres
// - Deve iniciar com letra maiúscula
func isValidTitle(title string) bool {
	// Verificar tamanho mínimo
	if len(title) < 20 {
		return false
	}

	// Pegar primeiro caractere (rune) do título
	for _, r := range title {
		// Verificar se é letra maiúscula
		return unicode.IsUpper(r)
	}

	return false
}

// isIgnorableLink filtra links irrelevantes
func isIgnorableLink(href string) bool {
	ignoredPatterns := []string{
		"unsubscribe",
		"preferences",
		"mailto:",
		"tel:",
		"facebook.com/sharer",
		"twitter.com/intent",
		"twitter.com/share",
		"linkedin.com/sharing",
		"linkedin.com/shareArticle",
		"pinterest.com/pin",
		"reddit.com/submit",
		"wa.me",
		"whatsapp.com",
		"t.me",
		"#",
	}

	lowerHref := strings.ToLower(href)

	// Ignorar fragmentos e âncoras
	if strings.HasPrefix(lowerHref, "#") {
		return true
	}

	// Ignorar links muito curtos
	if len(href) < 10 {
		return true
	}

	// Verificar padrões ignorados
	for _, pattern := range ignoredPatterns {
		if strings.Contains(lowerHref, pattern) {
			return true
		}
	}

	return false
}

// extractTitleFromLink extrai o título do link usando múltiplas estratégias
func extractTitleFromLink(s *goquery.Selection, href string, parsedURL *url.URL) string {
	// Função auxiliar para limpar título
	cleanTitle := func(t string) string {
		t = strings.ReplaceAll(t, "\n", " ")
		t = strings.ReplaceAll(t, "\r", " ")
		t = strings.ReplaceAll(t, "\t", " ")
		for strings.Contains(t, "  ") {
			t = strings.ReplaceAll(t, "  ", " ")
		}
		return strings.TrimSpace(t)
	}

	// Função para verificar se é um título válido
	isValidTitle := func(t string) bool {
		if t == "" || t == href {
			return false
		}
		// Ignorar se for só URL
		if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
			return false
		}
		// Ignorar se for muito curto (menos de 3 caracteres)
		if len(t) < 3 {
			return false
		}
		// Ignorar se for só números ou caracteres especiais
		hasLetter := false
		for _, r := range t {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127 {
				hasLetter = true
				break
			}
		}
		return hasLetter
	}

	var title string

	// 1. Para links do Medium, tentar extrair título da URL
	if strings.Contains(parsedURL.Host, "medium.com") {
		title = extractTitleFromMediumURL(href)
		if isValidTitle(title) {
			return cleanTitle(title)
		}
	}

	// 2. Procurar título em elementos ancestrais (pai, avô, etc.)
	// Medium e outras newsletters colocam o título em elementos pai
	parent := s.Parent()
	for i := 0; i < 5; i++ { // Subir até 5 níveis
		if parent.Length() == 0 {
			break
		}

		// Procurar h1, h2, h3, h4, strong dentro do contexto pai
		parent.Find("h1, h2, h3, h4, strong").Each(func(j int, heading *goquery.Selection) {
			if !isValidTitle(title) {
				headingText := cleanTitle(heading.Text())
				if isValidTitle(headingText) && len(headingText) > len(title) {
					title = headingText
				}
			}
		})

		if isValidTitle(title) {
			break
		}

		parent = parent.Parent()
	}

	if isValidTitle(title) {
		return title
	}

	// 3. Tentar pegar texto direto do link
	linkText := cleanTitle(s.Text())
	if isValidTitle(linkText) {
		return linkText
	}

	// 4. Procurar em elementos filhos
	s.Find("h1, h2, h3, h4, h5, strong, b, span, p").Each(func(j int, child *goquery.Selection) {
		if !isValidTitle(title) {
			childText := cleanTitle(child.Text())
			if isValidTitle(childText) {
				title = childText
			}
		}
	})

	if isValidTitle(title) {
		return title
	}

	// 5. Atributo title do link
	if titleAttr, exists := s.Attr("title"); exists {
		titleAttr = cleanTitle(titleAttr)
		if isValidTitle(titleAttr) {
			return titleAttr
		}
	}

	// 6. Alt de imagem dentro do link
	if img := s.Find("img"); img.Length() > 0 {
		if alt, exists := img.Attr("alt"); exists {
			alt = cleanTitle(alt)
			if isValidTitle(alt) {
				return alt
			}
		}
	}

	// 7. Último recurso: extrair título da URL
	title = extractTitleFromURL(href)
	if isValidTitle(title) {
		return title
	}

	return href
}

// extractTitleFromMediumURL extrai o título de uma URL do Medium
func extractTitleFromMediumURL(href string) string {
	// URLs do Medium têm formato: https://medium.com/.../titulo-do-artigo-abc123
	// ou https://medium.com/p/abc123

	parsedURL, err := url.Parse(href)
	if err != nil {
		return ""
	}

	path := parsedURL.Path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}

	// Pegar a última parte que geralmente é o slug do título
	slug := parts[len(parts)-1]

	// Ignorar se for só um ID (como "p" seguido de ID)
	if slug == "p" || len(slug) < 10 {
		if len(parts) > 1 {
			slug = parts[len(parts)-2]
		}
	}

	// Remover ID no final (geralmente últimos 12 caracteres após último hífen)
	if idx := strings.LastIndex(slug, "-"); idx > 0 && len(slug)-idx <= 13 {
		slug = slug[:idx]
	}

	// Converter hífens em espaços e capitalizar
	words := strings.Split(slug, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// extractTitleFromURL extrai um título legível de qualquer URL
func extractTitleFromURL(href string) string {
	parsedURL, err := url.Parse(href)
	if err != nil {
		return href
	}

	path := parsedURL.Path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return parsedURL.Host
	}

	parts := strings.Split(path, "/")
	slug := parts[len(parts)-1]

	// Remover extensão de arquivo
	if idx := strings.LastIndex(slug, "."); idx > 0 {
		slug = slug[:idx]
	}

	// Remover IDs e hashes comuns no final
	slug = regexp.MustCompile(`[-_][a-f0-9]{8,}$`).ReplaceAllString(slug, "")

	// Converter separadores em espaços
	slug = strings.ReplaceAll(slug, "-", " ")
	slug = strings.ReplaceAll(slug, "_", " ")

	// Capitalizar palavras
	words := strings.Fields(slug)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	result := strings.Join(words, " ")
	if result == "" {
		return parsedURL.Host
	}

	return result
}
