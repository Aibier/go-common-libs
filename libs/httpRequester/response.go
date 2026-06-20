package httpRequester

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ParseJSONResponse parses the response body as JSON into the provided target.
func ParseJSONResponse(resp *http.Response, target any) error {
	if resp == nil {
		return errors.New("response is nil")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return nil
}

// ExtractRawResponse returns the raw response body as bytes.
func ExtractRawResponse(resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("response is nil")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// IsSuccess checks if the response status code indicates success (2xx).
func IsSuccess(resp *http.Response) bool {
	return resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
}
