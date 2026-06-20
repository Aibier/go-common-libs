package middleware

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const requestIdentifierKey = "unique_request_uuid"

// RequestLoggerConfig defines optional configurations for the logger.
type RequestLoggerConfig struct {
	LogRequestBody  bool // Whether to log the request body
	LogResponseBody bool // Whether to log the response body
}

type responseRecorder struct {
	gin.ResponseWriter
	Body *bytes.Buffer
}

// write captures response data and writes it to the buffer and the response writer.
func (r *responseRecorder) write(data []byte) (int, error) {
	r.Body.Write(data)
	return r.ResponseWriter.Write(data)
}

// writeString captures response string and writes it to the buffer and the response writer.
func (r *responseRecorder) writeString(s string) (int, error) {
	r.Body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

// RequestLogger logs the details of each HTTP request and response in a Gin application.
func RequestLogger(logger *zap.SugaredLogger, configs ...*RequestLoggerConfig) gin.HandlerFunc {
	// Use the provided config or default to the default configuration
	var config *RequestLoggerConfig
	if len(configs) > 0 && configs[0] != nil {
		config = configs[0]
	} else {
		config = &RequestLoggerConfig{LogRequestBody: true, LogResponseBody: true}
	}

	return func(c *gin.Context) {
		start := time.Now()

		// Generate a unique request ID and store it in the context
		requestID := uuid.New().String()
		c.Set(requestIdentifierKey, requestID)

		fields := prepareRequestLogFields(c, requestID, config, logger)
		recorder := &responseRecorder{
			ResponseWriter: c.Writer,
			Body:           &bytes.Buffer{},
		}
		c.Writer = recorder

		// Process the request
		c.Next()
		fields = append(fields, c.Writer.Status())
		fields = append(fields, fmt.Sprintf("(%s %s)", recorder.Header().Get("Content-Type"), recorder.Header().Get("Content-Length")))
		if config.LogResponseBody {
			fields = append(fields, fmt.Sprintf("{response: %s}", recorder.Body.String()))
		}
		fields = append(fields, "duration", time.Since(start))
		logger.Info(fields)

	}
}

// prepareRequestLogFields prepares the fields for request logging.
func prepareRequestLogFields(c *gin.Context, requestID string, config *RequestLoggerConfig, logger *zap.SugaredLogger) []interface{} {
	fields := []interface{}{
		c.Request.Method,
		c.Request.URL.String(),
	}

	// Log the request body if enabled in the config
	if config.LogRequestBody && c.Request.Body != nil {
		body, err := readAndRestoreBody(c)
		if err == nil {
			requestBody := strings.ReplaceAll(string(body), "\n", " ")
			requestBody = strings.TrimSpace(requestBody)
			fields = append(fields, fmt.Sprintf("{request: %s}", requestBody))
		} else {
			logger.Warnw("Failed to read request body", "request_id", requestID, "error", err)
		}
	}

	return fields
}

// readAndRestoreBody reads the request body and restores it for future use in Gin.
func readAndRestoreBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}

	// Read the body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}

	// Restore the body so Gin can process it later
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

// getRequestID retrieves the request ID from the Gin context.
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(requestIdentifierKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
