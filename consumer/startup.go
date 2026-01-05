// Package main provides startup utilities for service dependencies.
// This module handles waiting for Kafka and liquidity service readiness
// before starting the consumer service.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

// waitForKafka waits for Kafka to be ready by attempting connections.
// It uses exponential backoff and sets the kafkaReady flag when successful.
func waitForKafka(brokerAddr string, maxAttempts int) error {
	log.Printf("Waiting for Kafka at %s...", brokerAddr)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := kafka.Dial("tcp", brokerAddr)
		if err == nil {
			_, err = conn.Controller()
			conn.Close()
			if err == nil {
				log.Printf("Kafka is ready after %d attempts", attempt)
				atomic.StoreInt32(&kafkaReady, 1)
				return nil
			}
		}

		if attempt < maxAttempts {
			waitTime := time.Duration(attempt) * 2 * time.Second
			log.Printf("Kafka not ready (attempt %d/%d), retrying in %v...", attempt, maxAttempts, waitTime)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("kafka not ready after %d attempts", maxAttempts)
}

// waitForLiquidityService waits for Liquidity Service to be ready.
// It checks the HTTP readiness endpoint and uses exponential backoff.
func waitForLiquidityService(serviceAddr string, maxAttempts int) error {
	log.Printf("Waiting for Liquidity Service at %s...", serviceAddr)

	// Extract host for HTTP health check
	host := serviceAddr
	if len(serviceAddr) > 6 && serviceAddr[len(serviceAddr)-6:] == ":50051" {
		host = serviceAddr[:len(serviceAddr)-6] + ":8080"
	}

	healthURL := fmt.Sprintf("http://%s/ready", host)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Printf("Liquidity Service is ready after %d attempts", attempt)
					atomic.StoreInt32(&liquidityReady, 1)
					cancel()
					return nil
				}
			}
		}

		cancel()

		if attempt < maxAttempts {
			waitTime := time.Duration(attempt) * 2 * time.Second
			log.Printf("Liquidity Service not ready (attempt %d/%d), retrying in %v...", attempt, maxAttempts, waitTime)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("liquidity service not ready after %d attempts", maxAttempts)
}

// monitorDependencies continuously monitors dependency health
func monitorDependencies(ctx context.Context, brokerAddr, liquidityAddr string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check Kafka
			err := waitForKafka(brokerAddr, 1)
			if err != nil {
				log.Printf("Kafka health check failed: %v", err)
				atomic.StoreInt32(&kafkaReady, 0)
			}

			// Check Liquidity Service
			err = waitForLiquidityService(liquidityAddr, 1)
			if err != nil {
				log.Printf("Liquidity Service health check failed: %v", err)
				atomic.StoreInt32(&liquidityReady, 0)
			}
		}
	}
}
