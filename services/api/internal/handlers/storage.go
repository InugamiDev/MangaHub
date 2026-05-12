package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ChapterPageStorage interface {
	Save(ctx context.Context, mangaID, chapterID string, header *multipart.FileHeader) (string, error)
	SaveBytes(ctx context.Context, mangaID, chapterID, filename, contentType string, body []byte) (string, error)
}

type StorageConfig struct {
	Provider          string
	LocalDir          string
	R2Endpoint        string
	R2AccountID       string
	R2Bucket          string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2PublicBaseURL   string
	R2Region          string
}

type LocalChapterStorage struct {
	Dir string
}

type R2ChapterStorage struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	Region          string
	Client          *http.Client
}

func NewConfiguredChapterStorage(config StorageConfig) (ChapterPageStorage, error) {
	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	if provider == "" {
		if hasR2Config(config) {
			provider = "r2"
		} else {
			provider = "local"
		}
	}

	switch provider {
	case "local":
		if config.LocalDir == "" {
			config.LocalDir = "data/uploads/chapters"
		}
		return LocalChapterStorage{Dir: config.LocalDir}, nil
	case "r2":
		if config.R2Endpoint == "" && config.R2AccountID != "" {
			config.R2Endpoint = "https://" + config.R2AccountID + ".r2.cloudflarestorage.com"
		}
		if config.R2Region == "" {
			config.R2Region = "auto"
		}
		storage := &R2ChapterStorage{
			Endpoint:        strings.TrimRight(config.R2Endpoint, "/"),
			Bucket:          strings.TrimSpace(config.R2Bucket),
			AccessKeyID:     strings.TrimSpace(config.R2AccessKeyID),
			SecretAccessKey: strings.TrimSpace(config.R2SecretAccessKey),
			PublicBaseURL:   strings.TrimRight(strings.TrimSpace(config.R2PublicBaseURL), "/"),
			Region:          strings.TrimSpace(config.R2Region),
			Client:          &http.Client{Timeout: 30 * time.Second},
		}
		if err := storage.Validate(); err != nil {
			return nil, err
		}
		return storage, nil
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", config.Provider)
	}
}

func hasR2Config(config StorageConfig) bool {
	return config.R2Endpoint != "" ||
		config.R2AccountID != "" ||
		config.R2Bucket != "" ||
		config.R2AccessKeyID != "" ||
		config.R2SecretAccessKey != "" ||
		config.R2PublicBaseURL != ""
}

func (s LocalChapterStorage) Save(_ context.Context, mangaID, chapterID string, header *multipart.FileHeader) (string, error) {
	source, err := header.Open()
	if err != nil {
		return "", errors.New("failed to open uploaded page")
	}
	defer source.Close()

	body, err := io.ReadAll(source)
	if err != nil {
		return "", errors.New("failed to read uploaded page")
	}
	return s.SaveBytes(context.Background(), mangaID, chapterID, header.Filename, header.Header.Get("Content-Type"), body)
}

func (s LocalChapterStorage) SaveBytes(_ context.Context, mangaID, chapterID, filename, _ string, body []byte) (string, error) {
	if s.Dir == "" {
		s.Dir = "data/uploads/chapters"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedPageExtension(ext) {
		return "", fmt.Errorf("unsupported page file type %q", ext)
	}
	name := uuid.NewString() + ext
	dir := filepath.Join(s.Dir, sanitizePathPart(mangaID), sanitizePathPart(chapterID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errors.New("failed to prepare upload directory")
	}
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return "", errors.New("failed to store uploaded page")
	}
	return "/media/chapters/" + escapePathPart(sanitizePathPart(mangaID)) + "/" + escapePathPart(sanitizePathPart(chapterID)) + "/" + escapePathPart(name), nil
}

func (s *R2ChapterStorage) Validate() error {
	if strings.TrimSpace(s.Endpoint) == "" {
		return errors.New("R2_ENDPOINT or R2_ACCOUNT_ID is required for R2 storage")
	}
	if strings.TrimSpace(s.Bucket) == "" {
		return errors.New("R2_BUCKET is required for R2 storage")
	}
	if strings.TrimSpace(s.AccessKeyID) == "" {
		return errors.New("R2_ACCESS_KEY_ID is required for R2 storage")
	}
	if strings.TrimSpace(s.SecretAccessKey) == "" {
		return errors.New("R2_SECRET_ACCESS_KEY is required for R2 storage")
	}
	if _, err := url.ParseRequestURI(s.Endpoint); err != nil {
		return errors.New("R2_ENDPOINT must be a valid URL")
	}
	if s.Region == "" {
		s.Region = "auto"
	}
	if s.Client == nil {
		s.Client = &http.Client{Timeout: 30 * time.Second}
	}
	return nil
}

func (s *R2ChapterStorage) Save(ctx context.Context, mangaID, chapterID string, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedPageExtension(ext) {
		return "", fmt.Errorf("unsupported page file type %q", ext)
	}
	source, err := header.Open()
	if err != nil {
		return "", errors.New("failed to open uploaded page")
	}
	defer source.Close()

	body, err := io.ReadAll(source)
	if err != nil {
		return "", errors.New("failed to read uploaded page")
	}
	return s.SaveBytes(ctx, mangaID, chapterID, header.Filename, header.Header.Get("Content-Type"), body)
}

func (s *R2ChapterStorage) SaveBytes(ctx context.Context, mangaID, chapterID, filename, contentType string, body []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedPageExtension(ext) {
		return "", fmt.Errorf("unsupported page file type %q", ext)
	}
	key := path.Join("chapters", sanitizePathPart(mangaID), sanitizePathPart(chapterID), uuid.NewString()+ext)
	if contentType == "" {
		contentType = mime.TypeByExtension(ext)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := s.putObject(ctx, key, contentType, body); err != nil {
		return "", err
	}
	if s.PublicBaseURL != "" {
		return s.PublicBaseURL + "/" + escapePath(key), nil
	}
	return s.Endpoint + "/" + escapePathPart(s.Bucket) + "/" + escapePath(key), nil
}

func (s *R2ChapterStorage) putObject(ctx context.Context, key, contentType string, body []byte) error {
	endpoint, err := url.Parse(s.Endpoint)
	if err != nil {
		return errors.New("R2 endpoint is invalid")
	}
	endpoint.Path = "/" + escapePathPart(s.Bucket) + "/" + escapePath(key)

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := sha256Hex(body)
	canonicalURI := endpoint.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalHeaders := "content-type:" + contentType + "\n" +
		"host:" + endpoint.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := dateStamp + "/" + s.Region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(signingKey(s.SecretAccessKey, dateStamp, s.Region, "s3"), stringToSign))
	authHeader := "AWS4-HMAC-SHA256 Credential=" + s.AccessKeyID + "/" + scope + ", SignedHeaders=" + signedHeaders + ", Signature=" + signature

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return errors.New("failed to create R2 upload request")
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", amzDate)

	resp, err := s.Client.Do(req)
	if err != nil {
		return errors.New("failed to upload page to R2")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("R2 upload failed with status %d", resp.StatusCode)
	}
	return nil
}

func saveMultipartFile(header *multipart.FileHeader, target string) error {
	source, err := header.Open()
	if err != nil {
		return errors.New("failed to open uploaded page")
	}
	defer source.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return errors.New("failed to store uploaded page")
	}
	defer dst.Close()
	if _, err := io.Copy(dst, source); err != nil {
		return errors.New("failed to write uploaded page")
	}
	return nil
}

func signingKey(secret, dateStamp, region, service string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, service)
	return hmacSHA256(serviceKey, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write([]byte(value))
	return hash.Sum(nil)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for index, part := range parts {
		parts[index] = escapePathPart(part)
	}
	return strings.Join(parts, "/")
}

func escapePathPart(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "+", "%20")
}
