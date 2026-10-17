package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Ограничения клиента
const (
	maxAttempts  = 3
	pageSize     = 100
	maxPages     = 100
	stallTimeout = 120 * time.Second
)

// Client клиент для работы с Partner API 1С:ГРМ
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client // обычные API-запросы (короткий таймаут)
	dlClient   *http.Client // скачивание файлов (без общего таймаута)
}

// Application приложение в 1С:ГРМ
type Application struct {
	ID                   string `json:"id"`
	ConfigurationID      string `json:"configurationId"`
	ConfigurationName    string `json:"configurationName"`
	ConfigurationVersion string `json:"configurationVersionId"`
	PlatformVersionID    string `json:"platformVersionId"`
	PlatformVersion      string `json:"platformVersion"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	URL                  string `json:"url"`
	LicenseCount         int    `json:"licenseCount"`
	CustomerID           string `json:"customerId"`
	Tariff               string `json:"tariff"`
	BillingState         string `json:"billingState"`
	Comment              string `json:"comment"`
	DiskQuota            int64  `json:"diskQuota"`
}

// Backup бэкап приложения
type Backup struct {
	ID         string  `json:"id"`
	Date       string  `json:"date"`
	Size       float64 `json:"size"`
	BackupType string  `json:"backupType"`
	Link       string  `json:"link"`
	Status     string  `json:"status"`
}

// Customer клиент (юрлицо/ИП)
type Customer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	INN  string `json:"inn"`
}

// New создает новый клиент API
func New(baseURL, token string, timeoutSec int) *Client {
	if timeoutSec < 30 {
		timeoutSec = 30
	}
	dlTransport := http.DefaultTransport.(*http.Transport).Clone()
	dlTransport.ResponseHeaderTimeout = 60 * time.Second
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		dlClient: &http.Client{Transport: dlTransport},
	}
}

// newRequest создает GET-запрос с авторизацией
func (c *Client) newRequest(ctx context.Context, fullURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// readAPIError извлекает сообщение из тела ответа с ошибкой
func readAPIError(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := ""
	var m struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &m); err == nil && m.Message != "" {
		msg = m.Message
	} else if len(body) > 0 {
		msg = strings.TrimSpace(string(body))
	}
	if len(msg) > 300 {
		msg = msg[:300]
	}
	if msg == "" {
		return fmt.Errorf("API error: статус %d", resp.StatusCode)
	}
	return fmt.Errorf("API error: статус %d: %s", resp.StatusCode, msg)
}

// get выполняет GET с повторами при временных ошибках; возвращает тело ответа
func (c *Client) get(ctx context.Context, fullURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}
		req, err := c.newRequest(ctx, fullURL)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}
			return body, nil
		}
		apiErr := readAPIError(resp)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = apiErr
			continue
		}
		return nil, apiErr
	}
	return nil, fmt.Errorf("после %d попыток: %w", maxAttempts, lastErr)
}

// decodeItems поддерживает как массив, так и объект {"content": [...]}
func decodeItems[T any](data []byte) ([]T, error) {
	var items []T
	if err := json.Unmarshal(data, &items); err == nil {
		return items, nil
	}
	var wrapper struct {
		Content []T `json:"content"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Content, nil
}

// GetApplications получает список приложений (с пагинацией)
func (c *Client) GetApplications(ctx context.Context) ([]Application, error) {
	var all []Application
	for page := 0; page < maxPages; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("size", strconv.Itoa(pageSize))
		body, err := c.get(ctx, c.baseURL+"/applications?"+q.Encode())
		if err != nil {
			return nil, err
		}
		items, err := decodeItems[Application](body)
		if err != nil {
			return nil, fmt.Errorf("ошибка разбора списка приложений: %w", err)
		}
		all = append(all, items...)
		if len(items) < pageSize {
			break
		}
	}
	return all, nil
}

// GetBackups получает список бэкапов приложения (с пагинацией)
func (c *Client) GetBackups(ctx context.Context, appID string) ([]Backup, error) {
	var all []Backup
	for page := 0; page < maxPages; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("size", strconv.Itoa(pageSize))
		body, err := c.get(ctx, c.baseURL+"/applications/"+appID+"/backups?"+q.Encode())
		if err != nil {
			return nil, err
		}
		items, err := decodeItems[Backup](body)
		if err != nil {
			return nil, fmt.Errorf("ошибка разбора списка бэкапов: %w", err)
		}
		all = append(all, items...)
		if len(items) < pageSize {
			break
		}
	}
	return all, nil
}

// GetCustomer получает информацию о клиенте
func (c *Client) GetCustomer(ctx context.Context, customerID string) (*Customer, error) {
	body, err := c.get(ctx, c.baseURL+"/customers/"+customerID)
	if err != nil {
		return nil, err
	}
	var cust Customer
	if err := json.Unmarshal(body, &cust); err == nil && cust.Name != "" {
		return &cust, nil
	}
	items, err := decodeItems[Customer](body)
	if err != nil || len(items) == 0 {
		return nil, fmt.Errorf("не удалось разобрать ответ /customers/%s", customerID)
	}
	return &items[0], nil
}

// DownloadBackup скачивает бэкап по ссылке в файл с базовым путём basePath (без расширения).
// Данные пишутся в basePath+".part"; после успешной загрузки файл получает расширение
// по фактическому формату (.zip/.bz2/.gz/.7z/.rar) и возвращается полный путь.
// При наличии .part выполняется докачка через заголовок Range.
// Сторожевой таймер: если данных нет 120 секунд — загрузка прерывается.
func (c *Client) DownloadBackup(ctx context.Context, link, basePath string, progress func(written, total int64)) (string, error) {
	partPath := basePath + ".part"

	var resumeFrom int64
	if st, err := os.Stat(partPath); err == nil && st.Size() > 0 {
		resumeFrom = st.Size()
	}

	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}

	resp, err := c.dlClient.Do(req)
	if err != nil {
		return "", err
	}

	var file *os.File
	var expected int64

	switch {
	case resumeFrom > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		resp.Body.Close()
		if err := os.Remove(partPath); err != nil {
			return "", fmt.Errorf("поврежденный файл .part не удаляется: %v", err)
		}
		return c.DownloadBackup(ctx, link, basePath, progress)
	case resumeFrom > 0 && resp.StatusCode == http.StatusPartialContent:
		file, err = os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			resp.Body.Close()
			return "", err
		}
		if resp.ContentLength > 0 {
			expected = resumeFrom + resp.ContentLength
		}
	case resumeFrom > 0 && resp.StatusCode == http.StatusOK:
		file, err = os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			resp.Body.Close()
			return "", err
		}
		resumeFrom = 0
		if resp.ContentLength > 0 {
			expected = resp.ContentLength
		}
	case resp.StatusCode == http.StatusOK:
		file, err = os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			resp.Body.Close()
			return "", err
		}
		if resp.ContentLength > 0 {
			expected = resp.ContentLength
		}
	default:
		return "", readAPIError(resp)
	}
	defer resp.Body.Close()
	defer file.Close()

	written := resumeFrom
	if progress != nil {
		progress(written, expected)
	}

	stall := time.AfterFunc(stallTimeout, cancel)
	pr := &progressReader{
		reader: resp.Body,
		onRead: func(n int) {
			stall.Reset(stallTimeout)
			written += int64(n)
			if progress != nil {
				progress(written, expected)
			}
		},
	}

	_, copyErr := io.Copy(file, pr)
	stall.Stop()
	if copyErr != nil {
		return "", fmt.Errorf("загрузка прервана: %w", copyErr)
	}
	if expected > 0 && written != expected {
		return "", fmt.Errorf("загрузка неполная: %d из %d байт", written, expected)
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	finalPath := basePath + detectExtension(partPath)
	if err := os.Rename(partPath, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}

// detectExtension определяет расширение файла по первым байтам содержимого.
func detectExtension(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ".zip"
	}
	defer f.Close()

	var b [8]byte
	n, _ := f.Read(b[:])
	if n >= 4 && b[0] == 'P' && b[1] == 'K' && b[2] == 0x03 && b[3] == 0x04 {
		return ".zip"
	}
	if n >= 3 && b[0] == 'B' && b[1] == 'Z' && b[2] == 'h' {
		return ".bz2"
	}
	if n >= 2 && b[0] == 0x1F && b[1] == 0x8B {
		return ".gz"
	}
	if n >= 4 && b[0] == '7' && b[1] == 'z' && b[2] == 0xBC && b[3] == 0xAF {
		return ".7z"
	}
	if n >= 4 && b[0] == 'R' && b[1] == 'a' && b[2] == 'r' && b[3] == '!' {
		return ".rar"
	}
	return ".zip"
}

// progressReader

// progressReader сообщает о каждом успешно прочитанном фрагменте
type progressReader struct {
	reader io.Reader
	onRead func(int)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.onRead != nil {
		pr.onRead(n)
	}
	return n, err
}
