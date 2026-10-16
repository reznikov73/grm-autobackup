package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client клиент для работы с Partner API 1С:ГРМ
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
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
	ID           string `json:"id"`
	Date         string `json:"date"`
	Size         float64 `json:"size"`
	BackupType   string `json:"backupType"`
	Link         string `json:"link"`
	Status       string `json:"status"`
}

// New создает новый клиент API
func New(baseURL, token string, timeoutSec int) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// GetApplications получает список приложений
func (c *Client) GetApplications(ctx context.Context) ([]Application, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/applications", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	var apps []Application
	if err := json.NewDecoder(resp.Body).Decode(&apps); err != nil {
		return nil, err
	}

	return apps, nil
}

// GetBackups получает список бэкапов приложения
func (c *Client) GetBackups(ctx context.Context, appID string) ([]Backup, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/applications/"+appID+"/backups", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	var backups []Backup
	if err := json.NewDecoder(resp.Body).Decode(&backups); err != nil {
		return nil, err
	}

	return backups, nil
}

// DownloadBackup скачивает бэкап по ссылке
func (c *Client) DownloadBackup(ctx context.Context, link, savePath string, progress func(int64, int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download error: status %d", resp.StatusCode)
	}

	out, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer out.Close()

	var total int64
	if resp.ContentLength > 0 {
		total = resp.ContentLength
	}

	written, err := io.Copy(out, &progressReader{Reader: resp.Body, total: total, callback: progress})
	if err != nil {
		return err
	}

	_ = written
	return nil
}

// progressReader обертка для отслеживания прогресса
type progressReader struct {
	io.Reader
	total    int64
	written  int64
	callback func(int64, int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.written += int64(n)
	if pr.callback != nil && pr.total > 0 {
		pr.callback(pr.written, pr.total)
	}
	return n, err
}
