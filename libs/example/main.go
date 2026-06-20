package main

import (
	"context"
	"database/sql"
	"fmt"
	"idempotency"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// PaymentRequest represents a card payment request
type PaymentRequest struct {
	CardNumber string
	Amount     float64
	Currency   string
	MerchantID string
}

// PaymentResult represents the result of a payment processing attempt
type PaymentResult struct {
	Status         string
	IdempotencyKey string
	ProcessedAt    time.Time
}

// PaymentProcessor handles card payment processing
type PaymentProcessor struct {
	idempotencyService *idempotency.IdempotencyService
	idGenerator        *idempotency.Generator
}

func NewPaymentProcessor(service *idempotency.IdempotencyService) *PaymentProcessor {
	return &PaymentProcessor{
		idempotencyService: service,
		idGenerator:        idempotency.NewGenerator(),
	}
}

// ProcessPayment handles a card payment with idempotency guarantees
func (p *PaymentProcessor) ProcessPayment(ctx context.Context, req PaymentRequest) (*PaymentResult, error) {
	// Generate unique idempotency key
	idempotencyKey, err := p.idGenerator.GenerateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate idempotency key: %w", err)
	}

	// Set expiration for 24 hours
	expiration := time.Now().Add(24 * time.Hour)

	// Try to set idempotency key
	status, err := p.idempotencyService.SetKey(ctx, "card-payments", idempotencyKey, &expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to set idempotency key: %w", err)
	}

	result := &PaymentResult{
		IdempotencyKey: idempotencyKey,
		ProcessedAt:    time.Now(),
	}

	switch status {
	case "SUCCEEDED":
		// This is a new payment request
		log.Printf("Processing new payment request: %s\n", idempotencyKey)
		if err := p.executePayment(req); err != nil {
			return nil, err
		}
		result.Status = "PROCESSED"
		return result, nil
	case "DUPLICATE":
		// This payment was already processed
		log.Printf("Payment request already processed: %s\n", idempotencyKey)
		result.Status = "DUPLICATE"
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected idempotency status: %s", status)
	}
}

func (p *PaymentProcessor) executePayment(req PaymentRequest) error {
	// Simulate payment processing
	log.Printf("Processing payment for card: %s, amount: %.2f %s\n",
		maskCardNumber(req.CardNumber),
		req.Amount,
		req.Currency)

	// Simulate API call delay
	time.Sleep(500 * time.Millisecond)

	return nil
}

func maskCardNumber(cardNumber string) string {
	if len(cardNumber) < 4 {
		return cardNumber
	}
	return fmt.Sprintf("****%s", cardNumber[len(cardNumber)-4:])
}

func runDemos(processor *PaymentProcessor) {
	ctx := context.Background()

	// Scenario 1: Single successful payment
	fmt.Println("\n=== Scenario 1: Single Payment Processing ===")
	payment1 := PaymentRequest{
		CardNumber: "4111111111111111",
		Amount:     100.00,
		Currency:   "USD",
		MerchantID: "MERCH001",
	}

	result1, err := processor.ProcessPayment(ctx, payment1)
	if err != nil {
		log.Printf("Failed to process payment: %v\n", err)
	} else {
		log.Printf("Payment processed: %+v\n", result1)
	}

	// Scenario 2: Duplicate payment attempt
	fmt.Println("\n=== Scenario 2: Duplicate Payment Prevention ===")
	result2, err := processor.ProcessPayment(ctx, payment1)
	if err != nil {
		log.Printf("Failed to process payment: %v\n", err)
	} else {
		log.Printf("Duplicate payment attempt: %+v\n", result2)
	}

	// Scenario 3: Concurrent payments
	fmt.Println("\n=== Scenario 3: Concurrent Payment Processing ===")
	payment2 := PaymentRequest{
		CardNumber: "5555555555554444",
		Amount:     250.00,
		Currency:   "EUR",
		MerchantID: "MERCH001",
	}

	results := make(chan *PaymentResult, 3)
	errors := make(chan error, 3)

	// Launch multiple concurrent payment attempts
	for i := 0; i < 3; i++ {
		go func(index int) {
			result, err := processor.ProcessPayment(ctx, payment2)
			if err != nil {
				errors <- fmt.Errorf("concurrent payment %d failed: %v", index, err)
				results <- nil
				return
			}
			results <- result
			errors <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < 3; i++ {
		result := <-results
		err := <-errors
		if err != nil {
			log.Printf("Error: %v\n", err)
		} else if result != nil {
			log.Printf("Concurrent payment result: %+v\n", result)
		}
	}

	// Scenario 4: High-volume test
	fmt.Println("\n=== Scenario 4: High-volume Payment Processing ===")
	processHighVolume(ctx, processor, 10)
}

func processHighVolume(ctx context.Context, processor *PaymentProcessor, count int) {
	successful := 0
	duplicates := 0
	failed := 0

	start := time.Now()

	for i := 0; i < count; i++ {
		payment := PaymentRequest{
			CardNumber: fmt.Sprintf("4111111111%04d", i),
			Amount:     float64(100 + i),
			Currency:   "USD",
			MerchantID: "MERCH001",
		}

		result, err := processor.ProcessPayment(ctx, payment)
		if err != nil {
			failed++
			continue
		}

		if result.Status == "PROCESSED" {
			successful++
		} else if result.Status == "DUPLICATE" {
			duplicates++
		}
	}

	duration := time.Since(start)
	tps := float64(count) / duration.Seconds()

	fmt.Printf("\nHigh-volume test results:\n")
	fmt.Printf("Total processed: %d\n", count)
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Duplicates: %d\n", duplicates)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Transactions per second: %.2f\n", tps)
}

func main() {
	// Connect to database
	db, err := sql.Open("mysql", "root:@tcp(localhost:3306)/testdb?parseTime=true")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create store factory
	factory, err := idempotency.CreateStoreFactory(idempotency.StoreFactoryConfig{
		Type: idempotency.StoreTypeMySQL,
		DB:   db,
	})
	if err != nil {
		log.Fatalf("Failed to create store factory: %v", err)
	}

	// Create idempotency service
	service, err := idempotency.NewIdempotencyService(factory)
	if err != nil {
		log.Fatalf("Failed to create idempotency service: %v", err)
	}

	// Create payment processor
	processor := NewPaymentProcessor(service)

	// Run demonstration scenarios
	runDemos(processor)
}
