package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Storage interface {
	Upload(ctx context.Context, bucket string, filename string, file io.Reader, contentType string) (string, error)
	Delete(ctx context.Context, bucket string, filename string) error
	GetURL(ctx context.Context, bucket string, filename string) (string, error)
}

type SupabaseStorage struct {
	baseURL   string
	publicURL string
	apiKey    string
}

func NewSupabaseStorage(baseURL, publicURL, apiKey string) *SupabaseStorage {
	return &SupabaseStorage{
		baseURL:   baseURL,
		publicURL: publicURL,
		apiKey:    apiKey,
	}
}

func (s *SupabaseStorage) Upload(ctx context.Context, bucket string, filename string, file io.Reader, contentType string) (string, error) {
	log.Printf("[SupabaseStorage.Upload] Starting upload - Bucket: %s, Filename: %s, ContentType: %s", bucket, filename, contentType)

	if filename == "" {
		filename = fmt.Sprintf("%s_%d", uuid.New().String(), time.Now().Unix())
		log.Printf("[SupabaseStorage.Upload] Generated filename: %s", filename)
	}

	baseURL := strings.TrimSuffix(s.baseURL, "/")
	var url string
	if strings.HasSuffix(baseURL, "/storage/v1") {
		url = fmt.Sprintf("%s/object/%s/%s", baseURL, bucket, filename)
	} else {
		url = fmt.Sprintf("%s/storage/v1/object/%s/%s", baseURL, bucket, filename)
	}
	log.Printf("[SupabaseStorage.Upload] Upload URL: %s", url)

	req, err := createUploadRequest(url, s.apiKey, file, contentType)
	if err != nil {
		log.Printf("[SupabaseStorage.Upload] Failed to create request: %v", err)
		return "", fmt.Errorf("creating upload request: %w", err)
	}
	log.Printf("[SupabaseStorage.Upload] Request created successfully")

	resp, err := executeRequest(ctx, req)
	if err != nil {
		log.Printf("[SupabaseStorage.Upload] Request execution failed: %v", err)
		return "", fmt.Errorf("executing upload request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[SupabaseStorage.Upload] Response status: %d", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		log.Printf("[SupabaseStorage.Upload] Upload failed - Status: %d, Body: %s", resp.StatusCode, bodyStr)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, bodyStr)
	}

	log.Println("[SupabaseStorage.Upload] Upload successful, generating public URL")
	return s.GetURL(ctx, bucket, filename)
}

func (s *SupabaseStorage) Delete(ctx context.Context, bucket string, filename string) error {
	baseURL := strings.TrimSuffix(s.baseURL, "/")
	var url string
	if strings.HasSuffix(baseURL, "/storage/v1") {
		url = fmt.Sprintf("%s/object/%s/%s", baseURL, bucket, filename)
	} else {
		url = fmt.Sprintf("%s/storage/v1/object/%s/%s", baseURL, bucket, filename)
	}

	req, err := createDeleteRequest(url, s.apiKey)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	resp, err := executeRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *SupabaseStorage) GetURL(ctx context.Context, bucket string, filename string) (string, error) {
	publicURL := strings.TrimSuffix(s.publicURL, "/")
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", publicURL, bucket, filename), nil
}
