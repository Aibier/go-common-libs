package httpRequester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"
)

// RequestConfig holds the configuration for an HTTP request.
type RequestConfig struct {
	Method       string            // HTTP method (GET, POST, etc.)
	URL          string            // URL of the request
	Headers      map[string]string // Custom headers
	QueryParams  map[string]string // Query parameters
	Body         any               // Request body
	FormData     map[string]string // Form data (optional)
	Timeout      time.Duration     // Timeout for the request
	MaxRetries   int               // Maximum retries
	RetryBackoff time.Duration     // Time to wait between retries
	DecodeTarget any               // Optional: Decode response into this struct
}

// Response represents the HTTP response.
type Response struct {
	StatusCode int            // HTTP status code
	Headers    http.Header    // Response headers
	Body       []byte         // Raw response body
	ParsedBody map[string]any // JSON-parsed response body (if applicable)
}

// HttpRequest performs an HTTP request with retries, and decoding.
func HttpRequest(ctx context.Context, config RequestConfig) (*Response, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	var err error
	var response *http.Response
	for attempt := 1; attempt <= config.MaxRetries+1; attempt++ {
		response, err = doSingleRequest(ctx, client, config)
		if err == nil {
			break
		}
		log.Printf("Attempt %d failed: %v", attempt, err)
		time.Sleep(config.RetryBackoff)
	}
	if err != nil {
		return nil, fmt.Errorf("all retries failed: %w", err)
	}

	if response == nil {
		return nil, errors.New("response is nil")
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}(response.Body)

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var parsedBody map[string]any
	if err := json.Unmarshal(responseBody, &parsedBody); err != nil {
		parsedBody = nil // Ignore errors for non-JSON responses
	}

	if config.DecodeTarget != nil {
		if err := json.Unmarshal(responseBody, config.DecodeTarget); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return &Response{
		StatusCode: response.StatusCode,
		Headers:    response.Header,
		Body:       responseBody,
		ParsedBody: parsedBody,
	}, nil
}

func doSingleRequest(ctx context.Context, client *http.Client, config RequestConfig) (*http.Response, error) {
	bodyReader, err := buildRequestBody(config)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	addHeaders(request, config.Headers)
	addQueryParams(request, config.QueryParams)

	return client.Do(request)
}

func buildRequestBody(config RequestConfig) (io.Reader, error) {
	if config.FormData != nil {
		return createFormDataBody(config.FormData, config.Headers)
	} else if config.Body != nil {
		return createJSONBody(config.Body, config.Headers)
	}
	return nil, nil
}

func createFormDataBody(formData map[string]string, headers map[string]string) (io.Reader, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, value := range formData {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field %s: %w", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close form-data writer: %w", err)
	}

	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = writer.FormDataContentType()
	return &buf, nil
}

func createJSONBody(body any, headers map[string]string) (io.Reader, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/json"
	return bytes.NewBuffer(bodyBytes), nil
}

func addHeaders(request *http.Request, headers map[string]string) {
	for key, value := range headers {
		request.Header.Set(key, value)
	}
}

func addQueryParams(request *http.Request, queryParams map[string]string) {
	query := request.URL.Query()
	for key, value := range queryParams {
		query.Add(key, value)
	}
	request.URL.RawQuery = query.Encode()
}
