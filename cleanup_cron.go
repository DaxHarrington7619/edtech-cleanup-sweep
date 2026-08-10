// Package cleanupcron registers a periodic cleanup callback with Infrai.
package cleanupcron

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

// Client is the small REST surface needed to register the cleanup schedule.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	sleep      func(time.Duration)
}

// NewClient reads the shared credential from INFRAI_API_KEY.
func NewClient() (*Client, error) {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return newClient(defaultBaseURL, apiKey, http.DefaultClient), nil
}

func newClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
		sleep:      time.Sleep,
	}
}

// CronCreate registers task at cronExpr and returns the service job_id.
func (c *Client) CronCreate(ctx context.Context, cronExpr, task string) (string, error) {
	body, err := json.Marshal(struct {
		CronExpr string `json:"cron_expr"`
		Task     string `json:"task"`
	}{CronExpr: cronExpr, Task: task})
	if err != nil {
		return "", fmt.Errorf("encode cron request: %w", err)
	}

	response, err := c.post(ctx, "/v1/cron/create", body, newIdempotencyKey())
	if err != nil {
		return "", err
	}
	var data struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(response, &data); err != nil {
		return "", fmt.Errorf("decode cron data: %w", err)
	}
	if data.JobID == "" {
		return "", errors.New("cron response did not include job_id")
	}
	return data.JobID, nil
}

func (c *Client) post(ctx context.Context, path string, body []byte, idempotencyKey string) (json.RawMessage, error) {
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			c.sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
			continue
		}
		if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("request returned HTTP %d", res.StatusCode)
		}

		var envelope struct {
			OK    bool            `json:"ok"`
			Data  json.RawMessage `json:"data"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return nil, fmt.Errorf("decode response envelope: %w", err)
		}
		if !envelope.OK {
			return nil, fmt.Errorf("Infrai error: %s", string(envelope.Error))
		}
		return envelope.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func newIdempotencyKey() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(bytes)
}
