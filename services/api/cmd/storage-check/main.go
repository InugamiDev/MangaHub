package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mangahub/services/api/internal/handlers"
)

func main() {
	loadDotEnv(".env", filepath.Join("..", "..", ".env"))

	storage, err := handlers.NewConfiguredChapterStorage(handlers.StorageConfig{
		Provider:          "r2",
		R2Endpoint:        getenv("R2_ENDPOINT", ""),
		R2AccountID:       getenv("R2_ACCOUNT_ID", ""),
		R2Bucket:          getenv("R2_BUCKET", ""),
		R2AccessKeyID:     getenv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getenv("R2_SECRET_ACCESS_KEY", ""),
		R2PublicBaseURL:   getenv("R2_PUBLIC_BASE_URL", ""),
		R2Region:          getenv("R2_REGION", ""),
	})
	if err != nil {
		log.Fatalf("configure R2 storage: %v", err)
	}

	header, cleanup, err := testImageFileHeader()
	if err != nil {
		log.Fatalf("prepare test image: %v", err)
	}
	defer cleanup()

	pageURL, err := storage.Save(context.Background(), "r2-smoke-test", "chapter-"+time.Now().UTC().Format("20060102T150405Z"), header)
	if err != nil {
		log.Fatalf("upload to R2: %v", err)
	}
	log.Println("R2 upload ok")

	if getenv("R2_PUBLIC_BASE_URL", "") == "" {
		log.Println("R2 public URL check skipped: R2_PUBLIC_BASE_URL is empty")
		return
	}
	if err := verifyPublicURL(pageURL); err != nil {
		log.Fatalf("verify R2 public URL: %v", err)
	}
	log.Println("R2 public URL ok")
}

func testImageFileHeader() (*multipart.FileHeader, func(), error) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lJgQ9wAAAABJRU5ErkJggg==")
	if err != nil {
		return nil, func() {}, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("pages", "r2-smoke.png")
	if err != nil {
		return nil, func() {}, err
	}
	if _, err := part.Write(png); err != nil {
		return nil, func() {}, err
	}
	if err := writer.Close(); err != nil {
		return nil, func() {}, err
	}

	req, err := http.NewRequest(http.MethodPost, "/smoke", &body)
	if err != nil {
		return nil, func() {}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		return nil, func() {}, err
	}
	files := req.MultipartForm.File["pages"]
	if len(files) == 0 {
		return nil, func() {}, errors.New("test multipart file was not created")
	}
	cleanup := func() {
		_ = req.MultipartForm.RemoveAll()
	}
	return files[0], cleanup, nil
}

func verifyPublicURL(pageURL string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return errors.New("uploaded page URL is invalid")
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("public page URL is not reachable")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("public page URL returned status %d", resp.StatusCode)
	}
	return nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func loadDotEnv(paths ...string) {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		_ = file.Close()
	}
}
