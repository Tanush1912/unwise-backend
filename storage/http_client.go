package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func createUploadRequest(url, apiKey string, file io.Reader, contentType string) (*http.Request, error) {
	body := &bytes.Buffer{}
	if _, err := io.Copy(body, file); err != nil {
		return nil, fmt.Errorf("copying file to buffer: %w", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	return req, nil
}

func createDeleteRequest(url, apiKey string) (*http.Request, error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	return req, nil
}

func executeRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}
