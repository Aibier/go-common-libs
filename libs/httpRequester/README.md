
# HTTP Requester Library

The `httpRequester` library is a reusable Go module for making HTTP requests with support for:

1. **Retry Mechanism**: Configurable retry count and backoff duration.
2. **Timeout**: Set a timeout for the entire request.
3. **JSON Decoding**: Automatically decodes JSON responses into a map or a custom struct.
4. **Form Data Support**: Create multipart form-data requests.
5. **Custom Headers**: Add any headers as required for your API.

## Configuration Parameters

- `Method`: HTTP method (GET, POST, etc.)
- `URL`: The endpoint URL.
- `Headers`: Optional headers for the request.
- `QueryParams`: Optional query parameters.
- `Body`: Request body for JSON payloads.
- `FormData`: For form-data submissions.
- `Timeout`: Timeout duration for the request.
- `MaxRetries`: Number of retry attempts.
- `RetryBackoff`: Duration to wait between retries.
- `DecodeTarget`: Pointer to a struct for automatic JSON decoding.


## Response Object

- `StatusCode`: HTTP status code of the response.
- `Headers`: Response headers.
- `Body`: Raw response body.
- `ParsedBody`: JSON-parsed body as a map, if applicable.


## Usage

### POST Request with JSON Body

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Aibier/go-common-libs/httpRequester"
)

func main() {
	config := httpRequester.RequestConfig{
		Method:  "POST",
		URL:     "https://jsonplaceholder.typicode.com/posts",
		Body: map[string]string{
			"title":  "foo",
			"body":   "bar",
			"userId": "1",
		},
		Timeout: 5 * time.Second,
	}

	ctx := context.Background()
	response, err := httpRequester.HttpRequest(ctx, config)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	fmt.Printf("Status Code: %d\n", response.StatusCode)
	fmt.Printf("Parsed Body: %+v\n", response.ParsedBody)
}
```
```azure
Status Code: 201
Parsed Body: map[id:101]
```

### Request with Form Data

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Aibier/go-common-libs/httpRequester"
)

func main() {
	config := httpRequester.RequestConfig{
		Method: "POST",
		URL:    "https://postman-echo.com/post",
		FormData: map[string]string{
			"key": "value",
		},
		Timeout: 10 * time.Second,
	}

	ctx := context.Background()
	response, err := httpRequester.HttpRequest(ctx, config)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	fmt.Printf("Status Code: %d\n", response.StatusCode)
}
```
```azure
Status Code: 200
```






