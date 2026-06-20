## RequestLogger Middleware for Gin Framework

RequestLogger is a customizable middleware for the Gin framework that logs details of incoming HTTP requests and their responses. It allows optional configuration to include request bodies in the logs.

## Features
1)  Automatically generates and logs a unique request ID for each request. 
2) Logs key request details such as HTTP method, URL, and remote IP. 
3) Optionally logs the request body (if enabled). 
4) Logs response status and duration for request processing


### Example Usage

```go
package main

import (
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
    "github.com/Aibier/go-common-libs/middleware"
)


func main() {
    // Initialize Gin router
    router := gin.Default()

    // Initialize Zap logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    sugaredLogger := logger.Sugar()

    // Add the middleware
    router.Use(middleware.RequestLogger(sugaredLogger))

    // Example routes
    router.GET("/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "pong"})
    })

    // Start the server
    router.Run(":8080")
}
```

## Configuration
The middleware accepts an optional RequestLoggerConfig object. This allows you to enable or disable logging of the request body.
```go
router.Use(middleware.RequestLogger(sugaredLogger, &middleware.RequestLoggerConfig{
    LogRequestBody: true,
	LogResponseBody: false
}))
```
Default Behavior: If no configuration is provided, the middleware defaults to LogRequestBody: true and LogResponseBody: true.
