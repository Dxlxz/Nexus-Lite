// Package main provides health check functionality for the producer service.
// This includes HTTP endpoints for liveness/readiness checks and Kafka connectivity monitoring.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

// HealthStatus represents the current health state of the producer service.
type HealthStatus struct {
	Status           string    `json:"status"`            // "healthy" or "degraded"
	Service          string    `json:"service"`           // Service name ("producer")
	Timestamp        time.Time `json:"timestamp"`         // Current timestamp
	MessagesProduced int64     `json:"messages_produced"` // Total messages produced
	Uptime           string    `json:"uptime"`            // Service uptime duration
	KafkaConnected   bool      `json:"kafka_connected"`   // Kafka connectivity status
}

// ReadinessStatus represents the readiness state for Kubernetes-style health checks.
type ReadinessStatus struct {
	Ready        bool      `json:"ready"`         // Overall readiness
	Service      string    `json:"service"`       // Service name
	Timestamp    time.Time `json:"timestamp"`     // Current timestamp
	KafkaReady   bool      `json:"kafka_ready"`   // Kafka readiness
	ConfigLoaded bool      `json:"config_loaded"` // Configuration loaded
}

var (
	healthStartTime time.Time
	kafkaHealthy    int32
	configLoaded    int32
)

func init() {
	healthStartTime = time.Now()
}

// handleHealth returns liveness status - indicates if the service is running and responsive.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:           "healthy",
		Service:          "producer",
		Timestamp:        time.Now(),
		MessagesProduced: atomic.LoadInt64(&metrics.MessagesProduced),
		Uptime:           time.Since(healthStartTime).String(),
		KafkaConnected:   atomic.LoadInt32(&kafkaHealthy) == 1,
	}

	w.Header().Set("Content-Type", "application/json")

	if !status.KafkaConnected {
		w.WriteHeader(http.StatusServiceUnavailable)
		status.Status = "degraded"
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(status)
}

// handleReady returns readiness status - indicates if the service is ready to serve traffic.
// Checks both Kafka connectivity and configuration loading status.
func handleReady(w http.ResponseWriter, r *http.Request) {
	kafkaReady := atomic.LoadInt32(&kafkaHealthy) == 1
	configReady := atomic.LoadInt32(&configLoaded) == 1
	ready := kafkaReady && configReady

	status := ReadinessStatus{
		Ready:        ready,
		Service:      "producer",
		Timestamp:    time.Now(),
		KafkaReady:   kafkaReady,
		ConfigLoaded: configReady,
	}

	w.Header().Set("Content-Type", "application/json")

	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// startHealthServer starts HTTP server for health checks on the specified address.
// Runs in a separate goroutine to avoid blocking the main service.
func startHealthServer(addr string) {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/ready", handleReady)

	go func() {
		log.Printf("Health check server starting on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Health server error: %v", err)
		}
	}()
}

// checkKafkaHealth performs Kafka health check by attempting to connect and get controller info.
// Returns an error if Kafka is unreachable or controller cannot be determined.
func checkKafkaHealth(brokerAddr string) error {
	conn, err := kafka.Dial("tcp", brokerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	_, err = conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	return nil
}

// monitorKafkaHealth continuously monitors Kafka health in the background.
// Updates the kafkaHealthy flag every 10 seconds for health check endpoints.
func monitorKafkaHealth(ctx context.Context, brokerAddr string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := checkKafkaHealth(brokerAddr)
			if err != nil {
				log.Printf("Kafka health check failed: %v", err)
				atomic.StoreInt32(&kafkaHealthy, 0)
			} else {
				atomic.StoreInt32(&kafkaHealthy, 1)
			}
		}
	}
}
