package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
)

// StorageClient handles Supabase Storage operations.
// Files can be encrypted with TEE keys before upload.
type StorageClient struct {
	client *Client
}

// =============================================================================
// Bucket Operations
// =============================================================================

// ListBuckets lists all storage buckets.
func (s *StorageClient) ListBuckets(ctx context.Context) ([]Bucket, error) {
	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "GET", s.client.storageURL+"/bucket", nil, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var buckets []Bucket
	if err := json.Unmarshal(respBody, &buckets); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return buckets, nil
}

// GetBucket retrieves a bucket by ID.
func (s *StorageClient) GetBucket(ctx context.Context, bucketID string) (*Bucket, error) {
	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "GET", s.client.storageURL+"/bucket/"+bucketID, nil, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var bucket Bucket
	if err := json.Unmarshal(respBody, &bucket); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &bucket, nil
}

// CreateBucket creates a new storage bucket.
func (s *StorageClient) CreateBucket(ctx context.Context, bucketID string, public bool) (*Bucket, error) {
	req := map[string]interface{}{
		"id":     bucketID,
		"name":   bucketID,
		"public": public,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", s.client.storageURL+"/bucket", body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var bucket Bucket
	if err := json.Unmarshal(respBody, &bucket); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &bucket, nil
}

// UpdateBucket updates a bucket.
func (s *StorageClient) UpdateBucket(ctx context.Context, bucketID string, public bool) (*Bucket, error) {
	req := map[string]interface{}{
		"public": public,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "PUT", s.client.storageURL+"/bucket/"+bucketID, body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var bucket Bucket
	if err := json.Unmarshal(respBody, &bucket); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &bucket, nil
}

// EmptyBucket empties a bucket.
func (s *StorageClient) EmptyBucket(ctx context.Context, bucketID string) error {
	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", s.client.storageURL+"/bucket/"+bucketID+"/empty", nil, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// DeleteBucket deletes a bucket.
func (s *StorageClient) DeleteBucket(ctx context.Context, bucketID string) error {
	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "DELETE", s.client.storageURL+"/bucket/"+bucketID, nil, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// =============================================================================
// File Operations
// =============================================================================

// Upload uploads a file to storage.
func (s *StorageClient) Upload(ctx context.Context, bucketID, filePath string, data []byte, opts *UploadOptions) (*FileObject, error) {
	urlStr := fmt.Sprintf("%s/object/%s/%s", s.client.storageURL, bucketID, url.PathEscape(filePath))

	headers := map[string]string{}
	if opts != nil {
		if opts.ContentType != "" {
			headers["Content-Type"] = opts.ContentType
		}
		if opts.CacheControl != "" {
			headers["Cache-Control"] = opts.CacheControl
		}
		if opts.Upsert {
			headers["x-upsert"] = "true"
		}
	}

	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/octet-stream"
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", urlStr, data, headers)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var result struct {
		Key string `json:"Key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &FileObject{
		Name:     path.Base(filePath),
		BucketID: bucketID,
	}, nil
}

// UploadWithToken uploads a file using a user's access token.
func (s *StorageClient) UploadWithToken(ctx context.Context, bucketID, filePath string, data []byte, opts *UploadOptions, accessToken string) (*FileObject, error) {
	urlStr := fmt.Sprintf("%s/object/%s/%s", s.client.storageURL, bucketID, url.PathEscape(filePath))

	headers := map[string]string{}
	if opts != nil {
		if opts.ContentType != "" {
			headers["Content-Type"] = opts.ContentType
		}
		if opts.CacheControl != "" {
			headers["Cache-Control"] = opts.CacheControl
		}
		if opts.Upsert {
			headers["x-upsert"] = "true"
		}
	}

	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/octet-stream"
	}

	respBody, statusCode, err := s.client.requestWithToken(ctx, "POST", urlStr, data, headers, accessToken)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	return &FileObject{
		Name:     path.Base(filePath),
		BucketID: bucketID,
	}, nil
}

// Download downloads a file from storage.
func (s *StorageClient) Download(ctx context.Context, bucketID, filePath string) ([]byte, error) {
	urlStr := fmt.Sprintf("%s/object/%s/%s", s.client.storageURL, bucketID, url.PathEscape(filePath))

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "GET", urlStr, nil, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	return respBody, nil
}

// DownloadWithToken downloads a file using a user's access token.
func (s *StorageClient) DownloadWithToken(ctx context.Context, bucketID, filePath, accessToken string) ([]byte, error) {
	urlStr := fmt.Sprintf("%s/object/%s/%s", s.client.storageURL, bucketID, url.PathEscape(filePath))

	respBody, statusCode, err := s.client.requestWithToken(ctx, "GET", urlStr, nil, nil, accessToken)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	return respBody, nil
}

// Delete deletes a file from storage.
func (s *StorageClient) Delete(ctx context.Context, bucketID string, filePaths []string) error {
	urlStr := fmt.Sprintf("%s/object/%s", s.client.storageURL, bucketID)

	req := map[string]interface{}{
		"prefixes": filePaths,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "DELETE", urlStr, body, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// List lists files in a bucket.
func (s *StorageClient) List(ctx context.Context, bucketID, prefix string, limit, offset int) ([]FileObject, error) {
	urlStr := fmt.Sprintf("%s/object/list/%s", s.client.storageURL, bucketID)

	req := map[string]interface{}{
		"prefix": prefix,
		"limit":  limit,
		"offset": offset,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", urlStr, body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var files []FileObject
	if err := json.Unmarshal(respBody, &files); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return files, nil
}

// Move moves a file to a new location.
func (s *StorageClient) Move(ctx context.Context, bucketID, fromPath, toPath string) error {
	urlStr := s.client.storageURL + "/object/move"

	req := map[string]interface{}{
		"bucketId":       bucketID,
		"sourceKey":      fromPath,
		"destinationKey": toPath,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", urlStr, body, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// Copy copies a file to a new location.
func (s *StorageClient) Copy(ctx context.Context, bucketID, fromPath, toPath string) error {
	urlStr := s.client.storageURL + "/object/copy"

	req := map[string]interface{}{
		"bucketId":       bucketID,
		"sourceKey":      fromPath,
		"destinationKey": toPath,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", urlStr, body, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// GetPublicURL returns the public URL for a file.
func (s *StorageClient) GetPublicURL(bucketID, filePath string) string {
	return fmt.Sprintf("%s/object/public/%s/%s", s.client.storageURL, bucketID, url.PathEscape(filePath))
}

// CreateSignedURL creates a signed URL for temporary access.
func (s *StorageClient) CreateSignedURL(ctx context.Context, bucketID, filePath string, expiresIn int) (string, error) {
	urlStr := fmt.Sprintf("%s/object/sign/%s/%s", s.client.storageURL, bucketID, url.PathEscape(filePath))

	req := map[string]interface{}{
		"expiresIn": expiresIn,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := s.client.requestWithServiceKey(ctx, "POST", urlStr, body, nil)
	if err != nil {
		return "", err
	}

	if statusCode >= 400 {
		return "", parseError(respBody, statusCode)
	}

	var result struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return s.client.baseURL + result.SignedURL, nil
}
