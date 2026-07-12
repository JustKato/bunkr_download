package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/net/html"
)

const (
	maxAlbumPageSize = 5 << 20
	bunkrAPIEndpoint = "https://apidl.bunkr.ru/api/_001_v2"
	bunkrDownloadRef = "https://get.bunkrr.su"
	bunkrCDNSignAPI  = "https://glb-apisign.cdn.cr/sign"
	httpUserAgent    = "BunkrDownloader/1.0"
)

var (
	albumSummaryPattern = regexp.MustCompile(`(?i)\(([^)]+)\)\s*(\d+)\s+files?`)
	albumFileField      = regexp.MustCompile(`(?m)^\s*([a-zA-Z]+):\s*(.+?)\s*,?\s*$`)
	allowedBunkrHosts   = map[string]struct{}{
		"bunkr.cr": {},
		"bunkr.fi": {},
		"bunkr.is": {},
		"bunkr.la": {},
		"bunkr.ps": {},
		"bunkr.ru": {},
		"bunkr.si": {},
	}
)

type bunkrMediaResponse struct {
	URL       string `json:"url"`
	Encrypted bool   `json:"encrypted"`
	Timestamp int64  `json:"timestamp"`
}

type cdnSignResponse struct {
	Token string `json:"token"`
	Ex    int64  `json:"ex"`
}

type BunkrService struct {
	client *http.Client

	mu               sync.RWMutex
	activeAlbum      *Album
	previewIndex     int
	outputFolder     string
	settings         AppSettings
	mediaURLCache    map[int64]string
	downloadCancel   context.CancelFunc
	downloadRunning  int32
	downloadProgress DownloadProgress
	cacheMu          sync.Mutex
	cacheInflight    map[int64]bool
}

type Album struct {
	URL       string      `json:"url"`
	Title     string      `json:"title"`
	TotalSize string      `json:"totalSize"`
	FileCount int         `json:"fileCount"`
	Files     []AlbumFile `json:"files"`
}

type AlbumFile struct {
	FileID     int64  `json:"fileID"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	MimeType   string `json:"mimeType"`
	Size       string `json:"size"`
	SizeBytes  int64  `json:"sizeBytes"`
	Date       string `json:"date"`
	PreviewURL string `json:"previewURL"`
	FileURL    string `json:"fileURL"`
}

func NewBunkrService() *BunkrService {
	s := &BunkrService{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				if !isBunkrHost(req.URL.Hostname()) {
					return fmt.Errorf("Bunkr redirected to an unsupported host")
				}
				return nil
			},
		},
		mediaURLCache: make(map[int64]string),
		cacheInflight: make(map[int64]bool),
	}
	settings, err := loadAppSettings()
	if err != nil {
		settings = defaultAppSettings()
	}
	s.applyLoadedSettings(settings)
	return s
}

func CanPreview(file AlbumFile) bool {
	switch strings.ToLower(file.Type) {
	case "image", "video":
		return true
	}
	mime := strings.ToLower(file.MimeType)
	if strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "video/") {
		return true
	}
	return strings.EqualFold(filepath.Ext(file.Name), ".pdf")
}

func (s *BunkrService) ValidateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("not a valid URL: %q", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if !isBunkrHost(u.Hostname()) {
		return "", fmt.Errorf("expected a supported Bunkr album URL")
	}
	if !strings.HasPrefix(strings.TrimSuffix(u.EscapedPath(), "/"), "/a/") {
		return "", fmt.Errorf("expected a Bunkr album URL with /a/")
	}
	return u.String(), nil
}

func (s *BunkrService) ScrapeAlbum(raw string) (*Album, error) {
	albumURL, err := s.ValidateURL(raw)
	if err != nil {
		return nil, err
	}

	advancedURL, err := withQueryParam(albumURL, "advanced", "1")
	if err != nil {
		return nil, err
	}

	body, pageURL, err := s.fetchPage(advancedURL)
	if err != nil {
		return nil, err
	}

	album, err := parseAlbumPage(pageURL, body)
	if err != nil {
		return nil, err
	}
	cleanURL := *pageURL
	cleanURL.RawQuery = ""
	cleanURL.Fragment = ""
	album.URL = cleanURL.String()

	s.mu.Lock()
	s.activeAlbum = album
	s.previewIndex = 0
	s.mu.Unlock()

	return album, nil
}

func (s *BunkrService) GetActiveAlbum() *Album {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activeAlbum == nil {
		return nil
	}
	copyAlbum := *s.activeAlbum
	copyAlbum.Files = append([]AlbumFile(nil), s.activeAlbum.Files...)
	return &copyAlbum
}

func (s *BunkrService) GetPreviewIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.previewIndex
}

func (s *BunkrService) SetPreviewIndex(index int) {
	s.mu.Lock()
	s.previewIndex = index
	s.mu.Unlock()
}

func (s *BunkrService) OpenPreview(startIndex int) error {
	app := application.Get()
	if app == nil {
		return fmt.Errorf("application not ready")
	}

	s.SetPreviewIndex(startIndex)

	if window, ok := app.Window.GetByName("preview"); ok {
		window.ExecJS(fmt.Sprintf("if(window.previewGoTo){window.previewGoTo(%d);}", startIndex))
		window.Focus()
		return nil
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "preview",
		Title:            "Preview",
		Width:            920,
		Height:           720,
		MinWidth:         640,
		MinHeight:        480,
		BackgroundColour: application.RGBA{Red: 38, Green: 42, Blue: 34, Alpha: 255},
		URL:              "/preview.html",
	})
	return nil
}

func (s *BunkrService) ResolveMediaURL(fileID int64) (string, error) {
	if fileID <= 0 {
		return "", fmt.Errorf("invalid file id")
	}

	s.mu.RLock()
	if cached, ok := s.mediaURLCache[fileID]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	mediaURL, err := s.resolveMediaURLUncached(fileID)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.mediaURLCache[fileID] = mediaURL
	s.mu.Unlock()
	return mediaURL, nil
}

func (s *BunkrService) resolveMediaURLUncached(fileID int64) (string, error) {
	referer := fmt.Sprintf("%s/file/%d", bunkrDownloadRef, fileID)
	payload, err := mediaAPIRequestBody(fileID)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequest(http.MethodPost, bunkrAPIEndpoint, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Referer", referer)
	request.Header.Set("Origin", bunkrDownloadRef)
	request.Header.Set("User-Agent", httpUserAgent)

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("resolving media URL: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		if len(body) > 0 {
			return "", fmt.Errorf("media API failed: %s (%s)", response.Status, strings.TrimSpace(string(body)))
		}
		return "", fmt.Errorf("media API failed: %s", response.Status)
	}

	var media bunkrMediaResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&media); err != nil {
		return "", fmt.Errorf("decoding media response: %w", err)
	}
	if media.URL == "" {
		return "", fmt.Errorf("media API returned an empty URL")
	}

	mediaURL := media.URL
	if media.Encrypted {
		key := fmt.Sprintf("SECRET_KEY_%d", media.Timestamp/3600)
		decrypted, err := decryptXOR(media.URL, key)
		if err != nil {
			return "", fmt.Errorf("decrypting media URL: %w", err)
		}
		mediaURL = decrypted
	}

	return s.maybeSignCDNURL(mediaURL)
}

func mediaAPIRequestBody(fileID int64) ([]byte, error) {
	return json.Marshal(map[string]string{
		"id": strconv.FormatInt(fileID, 10),
	})
}

func (s *BunkrService) maybeSignCDNURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing media URL: %w", err)
	}
	if !strings.Contains(parsed.Host, "cdn.cr") {
		return rawURL, nil
	}
	return s.signCDNURL(parsed)
}

func (s *BunkrService) signCDNURL(parsed *url.URL) (string, error) {
	if parsed.Scheme == "" || parsed.Host == "" || parsed.Path == "" {
		return "", fmt.Errorf("invalid CDN URL")
	}

	signRequestURL := bunkrCDNSignAPI + "?path=" + url.QueryEscape(parsed.Path)
	request, err := http.NewRequest(http.MethodGet, signRequestURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", httpUserAgent)

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("signing CDN URL: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("CDN sign API failed: %s", response.Status)
	}

	var signResp cdnSignResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&signResp); err != nil {
		return "", fmt.Errorf("decoding CDN sign response: %w", err)
	}
	if signResp.Token == "" || signResp.Ex == 0 {
		return "", fmt.Errorf("CDN sign API returned incomplete token data")
	}

	signed := *parsed
	query := signed.Query()
	query.Set("token", signResp.Token)
	query.Set("ex", strconv.FormatInt(signResp.Ex, 10))
	signed.RawQuery = query.Encode()
	return signed.String(), nil
}

func (s *BunkrService) fetchPage(pageURL string) (string, *url.URL, error) {
	request, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("creating album request: %w", err)
	}
	request.Header.Set("User-Agent", httpUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")

	response, err := s.client.Do(request)
	if err != nil {
		return "", nil, fmt.Errorf("loading album: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", nil, fmt.Errorf("album request failed: %s", response.Status)
	}
	if !isBunkrHost(response.Request.URL.Hostname()) {
		return "", nil, fmt.Errorf("album resolved to an unsupported host")
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, maxAlbumPageSize))
	if err != nil {
		return "", nil, fmt.Errorf("reading album page: %w", err)
	}
	return string(bodyBytes), response.Request.URL, nil
}

func parseAlbumPage(pageURL *url.URL, htmlContent string) (*Album, error) {
	album := &Album{}

	if title := extractMetaContent(htmlContent, `property="og:title" content="`); title != "" {
		album.Title = htmlUnescape(title)
	}
	if match := albumSummaryPattern.FindStringSubmatch(htmlContent); len(match) == 3 {
		album.TotalSize = match[1]
		fmt.Sscanf(match[2], "%d", &album.FileCount)
	}

	if files, err := parseAlbumFilesJS(pageURL, htmlContent); err == nil && len(files) > 0 {
		album.Files = files
	} else {
		document, err := html.Parse(strings.NewReader(htmlContent))
		if err != nil {
			return nil, fmt.Errorf("parsing album page: %w", err)
		}
		albumFromHTML, err := parseAlbumHTML(pageURL, document)
		if err != nil {
			return nil, err
		}
		if album.Title == "" {
			album.Title = albumFromHTML.Title
		}
		if album.TotalSize == "" {
			album.TotalSize = albumFromHTML.TotalSize
		}
		if album.FileCount == 0 {
			album.FileCount = albumFromHTML.FileCount
		}
		album.Files = albumFromHTML.Files
	}

	if album.Title == "" {
		album.Title = "Untitled Bunkr album"
	}
	if len(album.Files) == 0 {
		return nil, fmt.Errorf("no files found in this album; it may be private, unavailable, or use an unsupported page layout")
	}
	if album.FileCount == 0 {
		album.FileCount = len(album.Files)
	}
	return album, nil
}

func parseAlbumFilesJS(pageURL *url.URL, htmlContent string) ([]AlbumFile, error) {
	startMarker := "window.albumFiles = ["
	start := strings.Index(htmlContent, startMarker)
	if start == -1 {
		return nil, fmt.Errorf("window.albumFiles not found")
	}
	start += len(startMarker)

	endRel := strings.Index(htmlContent[start:], "];")
	if endRel == -1 {
		return nil, fmt.Errorf("window.albumFiles terminator not found")
	}

	block := strings.TrimSpace(htmlContent[start : start+endRel])
	if block == "" {
		return nil, fmt.Errorf("window.albumFiles is empty")
	}

	rawItems := regexp.MustCompile(`\n\s*},\s*\n`).Split(block, -1)
	files := make([]AlbumFile, 0, len(rawItems))
	for _, rawItem := range rawItems {
		rawItem = strings.TrimSpace(rawItem)
		rawItem = strings.TrimPrefix(rawItem, "{")
		rawItem = strings.TrimSuffix(rawItem, "}")
		rawItem = strings.TrimSpace(rawItem)
		if rawItem == "" {
			continue
		}

		fields := parseJSObjectFields(rawItem)
		fileID, _ := strconv.ParseInt(strings.TrimSpace(fields["id"]), 10, 64)
		slug := unquoteJS(fields["slug"])
		original := unquoteJS(fields["original"])
		if original == "" || slug == "" {
			continue
		}

		sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(fields["size"]), 10, 64)
		files = append(files, AlbumFile{
			FileID:     fileID,
			Name:       original,
			Type:       unquoteJS(fields["extension"]),
			MimeType:   unquoteJS(fields["type"]),
			Size:       formatBytes(sizeBytes),
			SizeBytes:  sizeBytes,
			Date:       unquoteJS(fields["timestamp"]),
			PreviewURL: unquoteJS(fields["thumbnail"]),
			FileURL:    resolveURL(pageURL, "/f/"+slug),
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no albumFiles entries parsed")
	}
	return files, nil
}

func parseJSObjectFields(raw string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "{" || line == "}," || line == "}" {
			continue
		}
		line = strings.TrimSuffix(line, ",")
		match := albumFileField.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		fields[match[1]] = strings.TrimSpace(match[2])
	}
	return fields
}

func parseAlbumHTML(pageURL *url.URL, document *html.Node) (*Album, error) {
	album := &Album{}
	walkElements(document, func(node *html.Node) {
		switch {
		case node.Data == "h1" && album.Title == "":
			album.Title = nodeText(node)
		case node.Data == "span" && hasClass(node, "font-semibold") && album.FileCount == 0:
			if match := albumSummaryPattern.FindStringSubmatch(nodeText(node)); len(match) == 3 {
				album.TotalSize = match[1]
				fmt.Sscanf(match[2], "%d", &album.FileCount)
			}
		case node.Data == "div" && hasClass(node, "theItem"):
			if file := parseAlbumFile(pageURL, node); file.Name != "" && file.FileURL != "" {
				album.Files = append(album.Files, file)
			}
		}
	})

	if len(album.Files) == 0 {
		return nil, fmt.Errorf("no HTML album items found")
	}
	return album, nil
}

func parseAlbumFile(pageURL *url.URL, item *html.Node) AlbumFile {
	file := AlbumFile{Name: strings.TrimSpace(attribute(item, "title"))}

	walkElements(item, func(node *html.Node) {
		switch {
		case node.Data == "span" && file.Type == "":
			for _, className := range classNames(node) {
				if strings.HasPrefix(className, "type-") {
					file.Type = strings.TrimPrefix(className, "type-")
					break
				}
			}
		case node.Data == "p":
			switch {
			case hasClass(node, "theName") && file.Name == "":
				file.Name = nodeText(node)
			case hasClass(node, "theSize"):
				file.Size = nodeText(node)
			}
		case hasClass(node, "theDate"):
			file.Date = nodeText(node)
		case (node.Data == "img" && hasClass(node, "grid-images_box-img")) ||
			(node.Data == "video" && file.PreviewURL == ""):
			if file.PreviewURL != "" {
				return
			}
			preview := attribute(node, "src")
			if preview == "" {
				preview = attribute(node, "poster")
			}
			file.PreviewURL = resolveURL(pageURL, preview)
		case node.Data == "a" && file.FileURL == "":
			file.FileURL = resolveURL(pageURL, attribute(node, "href"))
		}
	})

	if file.Type == "" {
		file.Type = "File"
	}
	return file
}

func decryptXOR(encryptedB64 string, key string) (string, error) {
	encrypted, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return "", err
	}
	keyBytes := []byte(key)
	result := make([]byte, len(encrypted))
	for i := range encrypted {
		result[i] = encrypted[i] ^ keyBytes[i%len(keyBytes)]
	}
	return string(result), nil
}

func withQueryParam(rawURL, key, value string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func extractMetaContent(htmlContent, prefix string) string {
	start := strings.Index(htmlContent, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(htmlContent[start:], `"`)
	if end == -1 {
		return ""
	}
	return htmlContent[start : start+end]
}

func htmlUnescape(value string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
	)
	return replacer.Replace(value)
}

func unquoteJS(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		inner := value[1 : len(value)-1]
		return strings.ReplaceAll(inner, `\"`, `"`)
	}
	return value
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "SIZE UNKNOWN"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.2f %s", float64(size)/float64(div), suffixes[exp])
}

func walkElements(node *html.Node, visit func(*html.Node)) {
	if node.Type == html.ElementNode {
		visit(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkElements(child, visit)
	}
}

func nodeText(node *html.Node) string {
	var text strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			text.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return strings.Join(strings.Fields(text.String()), " ")
}

func classNames(node *html.Node) []string {
	return strings.Fields(attribute(node, "class"))
}

func hasClass(node *html.Node, target string) bool {
	for _, className := range classNames(node) {
		if className == target {
			return true
		}
	}
	return false
}

func attribute(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func resolveURL(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func isBunkrHost(host string) bool {
	_, allowed := allowedBunkrHosts[strings.ToLower(host)]
	return allowed
}
