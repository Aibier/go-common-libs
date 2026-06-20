package idempotency

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"crypto/rand"
	"crypto/sha256"

	"github.com/oklog/ulid/v2"
)

type Generator struct {
	entropy *ulid.MonotonicEntropy
	mutex   sync.Mutex
}

// getContainerID tries different methods to get container ID
func getContainerID() string {
	// Try reading cgroup file for Docker container ID
	content, err := os.ReadFile("/proc/self/cgroup")
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "docker") {
				parts := strings.Split(line, "/")
				if len(parts) > 0 {
					containerID := parts[len(parts)-1]
					if len(containerID) >= 12 {
						return containerID[:12]
					}
					return containerID
				}
			}
		}
	}

	// Fallback to hostname if no container ID is found
	hostname, err := os.Hostname()
	if err == nil {
		return hostname
	}

	return "unknown"
}

// createEntropyWithContainerID creates an entropy source that incorporates the container ID
func createEntropyWithContainerID(containerID string) io.Reader {
	// Create a hash of the container ID
	hash := sha256.New()
	hash.Write([]byte(containerID))
	containerHash := hash.Sum(nil)

	// Create a custom reader that combines container hash with crypto/rand
	return io.MultiReader(
		strings.NewReader(string(containerHash)),
		rand.Reader,
	)
}

// NewGenerator creates a new ULID generator
func NewGenerator() *Generator {
	containerID := getContainerID()

	// Create entropy source with container ID
	entropySource := createEntropyWithContainerID(containerID)

	// Create monotonic entropy with our container-aware entropy source
	entropy := ulid.Monotonic(entropySource, 0)

	return &Generator{
		entropy: entropy,
	}
}

// GenerateID generates a new ULID
func (g *Generator) GenerateID() (string, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.entropy == nil {
		return "", fmt.Errorf("entropy source not initialized")
	}

	// Generate ULID using current timestamp
	id, err := ulid.New(ulid.Timestamp(time.Now()), g.entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate ULID: %w", err)
	}

	return id.String(), nil
}
