// Package main implements the ISO 20022 transaction consumer for Nexus-Lite.
// This service consumes ISO 20022 pacs.008 messages from Kafka, validates them,
// performs liquidity checks via gRPC, and broadcasts results via WebSocket.
package main

import (
	"context"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

// ISO 20022 pacs.008.001.08 - Full XSD Compliance Structure
type Document struct {
	XMLName           xml.Name       `xml:"Document"`
	Xmlns             string         `xml:"xmlns,attr"`
	XmlnsXsi          string         `xml:"xmlns:xsi,attr"`
	FIToFICstmrCdtTrf CreditTransfer `xml:"FIToFICstmrCdtTrf"`
}

type CreditTransfer struct {
	GrpHdr      GroupHeader       `xml:"GrpHdr"`
	CdtTrfTxInf []TransactionInfo `xml:"CdtTrfTxInf"`
}

type GroupHeader struct {
	MsgId    string         `xml:"MsgId"`
	CreDtTm  time.Time      `xml:"CreDtTm"`
	NbOfTxs  int            `xml:"NbOfTxs"`
	SttlmInf SettlementInfo `xml:"SttlmInf"`
	InstgAgt Agent          `xml:"InstgAgt"`
	InstdAgt Agent          `xml:"InstdAgt"`
}

type SettlementInfo struct {
	SttlmMtd string         `xml:"SttlmMtd"`
	ClrSys   ClearingSystem `xml:"ClrSys"`
}

type ClearingSystem struct {
	Prtry string `xml:"Prtry"`
}

type TransactionInfo struct {
	PmtId          PaymentID `xml:"PmtId"`
	IntrBkSttlmAmt Amount    `xml:"IntrBkSttlmAmt"`
	IntrBkSttlmDt  string    `xml:"IntrBkSttlmDt"`
	ChrgBr         string    `xml:"ChrgBr"`
	InstgAgt       Agent     `xml:"InstgAgt"`
	InstdAgt       Agent     `xml:"InstdAgt"`
	Dbtr           Party     `xml:"Dbtr"`
	DbtrAcct       Account   `xml:"DbtrAcct"`
	DbtrAgt        Agent     `xml:"DbtrAgt"`
	CdtrAgt        Agent     `xml:"CdtrAgt"`
	Cdtr           Party     `xml:"Cdtr"`
	CdtrAcct       Account   `xml:"CdtrAcct"`
}

type PaymentID struct {
	InstrId    string `xml:"InstrId"`
	EndToEndId string `xml:"EndToEndId"`
	UETR       string `xml:"UETR"`
}

type Party struct {
	Nm      string     `xml:"Nm"`
	PstlAdr PostalAddr `xml:"PstlAdr"`
}

type PostalAddr struct {
	Ctry string `xml:"Ctry"`
}

type Account struct {
	Id AccountID `xml:"Id"`
}

type AccountID struct {
	Othr OtherID `xml:"Othr"`
}

type OtherID struct {
	Id      string `xml:"Id"`
	SchmeNm Scheme `xml:"SchmeNm"`
}

type Scheme struct {
	Prtry string `xml:"Prtry"`
}

type Agent struct {
	FinInstnId FinancialInstitution `xml:"FinInstnId"`
}

type FinancialInstitution struct {
	BICFI string `xml:"BICFI"`
}

type Amount struct {
	Ccy   string `xml:"Ccy,attr"`
	Value string `xml:",chardata"`
}

// ValidationResult contains the outcome of processing an ISO 20022 message.
// It includes validation status, error details, and performance metrics.
type ValidationResult struct {
	Valid     bool          // Whether the message passed all validations
	ErrorCode string        // ISO 20022 error code (e.g., "AC04", "AM04")
	ErrorMsg  string        // Human-readable error message
	MsgId     string        // Message ID from the ISO 20022 message
	Timestamp time.Time     // When validation was performed
	Latency   time.Duration // Time taken to process the message
}

// Metrics tracks consumer performance and message processing statistics.
type Metrics struct {
	MessagesConsumed int64     // Total messages read from Kafka
	MessagesValid    int64     // Messages that passed validation
	MessagesInvalid  int64     // Messages that failed validation
	Errors           int64     // Processing errors (network, parsing, etc.)
	StartTime        time.Time // Service start time
	LastLogTime      time.Time // Last metrics log time
	LastMessageCount int64     // Message count at last log
}

var metrics Metrics

// Global liquidity client for gRPC communication with liquidity service
var liquidityClient *LiquidityClient

// Global readiness flags
var (
	kafkaReady     int32
	liquidityReady int32
)

// validateTransaction performs comprehensive validation of an ISO 20022 pacs.008 message.
// It checks XML structure, required fields, BIC formats, and business rules.
// Returns a ValidationResult with detailed error information if validation fails.
func validateTransaction(data []byte) ValidationResult {
	startTime := time.Now()
	result := ValidationResult{
		Valid:     true,
		Timestamp: time.Now(),
	}

	var doc Document
	err := xml.Unmarshal(data, &doc)
	if err != nil {
		result.Valid = false
		result.ErrorCode = "XML_PARSE_ERROR"
		result.ErrorMsg = fmt.Sprintf("Failed to parse XML: %v", err)
		result.Latency = time.Since(startTime)
		return result
	}

	// Validate Group Header
	if doc.FIToFICstmrCdtTrf.GrpHdr.MsgId == "" {
		result.Valid = false
		result.ErrorCode = "MISSING_MSG_ID"
		result.ErrorMsg = "Message ID is required"
	} else {
		result.MsgId = doc.FIToFICstmrCdtTrf.GrpHdr.MsgId
	}

	if doc.FIToFICstmrCdtTrf.GrpHdr.NbOfTxs <= 0 {
		result.Valid = false
		result.ErrorCode = "INVALID_TX_COUNT"
		result.ErrorMsg = "Transaction count must be greater than 0"
	}

	// Validate Settlement Method (must be one of: CLRG, INDA, INGA, COVE)
	sttlmMtd := doc.FIToFICstmrCdtTrf.GrpHdr.SttlmInf.SttlmMtd
	validSttlmMethods := map[string]bool{"CLRG": true, "INDA": true, "INGA": true, "COVE": true}
	if !validSttlmMethods[sttlmMtd] {
		result.Valid = false
		result.ErrorCode = "INVALID_STTLM_MTD"
		result.ErrorMsg = "Settlement Method must be one of: CLRG, INDA, INGA, COVE"
	}

	// Validate Transactions
	if len(doc.FIToFICstmrCdtTrf.CdtTrfTxInf) == 0 {
		result.Valid = false
		result.ErrorCode = "NO_TRANSACTIONS"
		result.ErrorMsg = "At least one transaction is required"
	}

	// Validate Debtor Agent BIC (should be 8 or 11 characters) - from first transaction
	if len(doc.FIToFICstmrCdtTrf.CdtTrfTxInf) > 0 {
		dbtrBIC := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].DbtrAgt.FinInstnId.BICFI
		if len(dbtrBIC) < 8 || len(dbtrBIC) > 11 {
			result.Valid = false
			result.ErrorCode = "INVALID_DBTR_BIC"
			result.ErrorMsg = "Debtor BIC must be 8 or 11 characters"
		}

		// Validate Creditor Agent BIC
		cdtrBIC := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].CdtrAgt.FinInstnId.BICFI
		if len(cdtrBIC) < 8 || len(cdtrBIC) > 11 {
			result.Valid = false
			result.ErrorCode = "INVALID_CDTR_BIC"
			result.ErrorMsg = "Creditor BIC must be 8 or 11 characters"
		}
	}

	// Valid charge bearer codes
	validChrgBr := map[string]bool{"DEBT": true, "CRED": true, "SHAR": true, "SLEV": true}

	for i, tx := range doc.FIToFICstmrCdtTrf.CdtTrfTxInf {
		if tx.PmtId.InstrId == "" {
			result.Valid = false
			result.ErrorCode = fmt.Sprintf("MISSING_PMT_ID_TX%d", i)
			result.ErrorMsg = fmt.Sprintf("Payment ID (InstrId) is required for transaction %d", i)
		}

		if tx.IntrBkSttlmAmt.Ccy == "" {
			result.Valid = false
			result.ErrorCode = fmt.Sprintf("MISSING_CURRENCY_TX%d", i)
			result.ErrorMsg = fmt.Sprintf("Currency is required for transaction %d", i)
		}

		if tx.IntrBkSttlmAmt.Value == "" {
			result.Valid = false
			result.ErrorCode = fmt.Sprintf("MISSING_AMOUNT_TX%d", i)
			result.ErrorMsg = fmt.Sprintf("Amount is required for transaction %d", i)
		}

		// Validate IntrBkSttlmDt (must not be empty)
		if tx.IntrBkSttlmDt == "" {
			result.Valid = false
			result.ErrorCode = fmt.Sprintf("MISSING_STTLM_DT_TX%d", i)
			result.ErrorMsg = fmt.Sprintf("Interbank Settlement Date is required for transaction %d", i)
		}

		// Validate ChrgBr (must be one of: DEBT, CRED, SHAR, SLEV)
		if !validChrgBr[tx.ChrgBr] {
			result.Valid = false
			result.ErrorCode = fmt.Sprintf("INVALID_CHRG_BR_TX%d", i)
			result.ErrorMsg = fmt.Sprintf("Charge Bearer must be one of: DEBT, CRED, SHAR, SLEV for transaction %d", i)
		}
	}

	result.Latency = time.Since(startTime)
	return result
}

// checkLiquidity performs a liquidity check via gRPC before transaction validation.
// It extracts bank IDs from BICs, checks available balances, and performs the transfer
// if sufficient funds are available. Returns approval status and error details.
func checkLiquidity(data []byte) (bool, string, string, float64) {
	var doc Document
	err := xml.Unmarshal(data, &doc)
	if err != nil {
		return false, "LIQUIDITY_CHECK_ERROR", "Failed to parse XML for liquidity check", 0
	}

	// Extract transaction details
	if len(doc.FIToFICstmrCdtTrf.CdtTrfTxInf) == 0 {
		return true, "", "", 0 // No transactions to check
	}

	tx := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0]
	sourceBIC := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].DbtrAgt.FinInstnId.BICFI
	destBIC := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].CdtrAgt.FinInstnId.BICFI
	sourceBankID := ExtractBankIDFromBIC(sourceBIC)
	destBankID := ExtractBankIDFromBIC(destBIC)

	// Parse amount
	amount, err := strconv.ParseFloat(tx.IntrBkSttlmAmt.Value, 64)
	if err != nil {
		return false, "LIQUIDITY_CHECK_ERROR", "Failed to parse amount", 0
	}

	currency := tx.IntrBkSttlmAmt.Ccy

	// Call liquidity service if client is available
	if liquidityClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Check and debit source bank
		approved, balance, errorCode, err := liquidityClient.CheckLiquidity(ctx, sourceBankID, amount, currency)
		if err != nil {
			log.Printf("Liquidity check failed for bank %s: %v", sourceBankID, err)
			// Continue with validation if liquidity check fails (fail-open)
			return true, "", "", 0
		}

		if !approved {
			return false, errorCode, fmt.Sprintf("Liquidity check rejected: %s", errorCode), balance
		}

		// Transaction approved - credit destination bank
		creditCtx, creditCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer creditCancel()

		success, newBalance, creditErr := liquidityClient.CreditBank(creditCtx, destBankID, amount, currency)
		if creditErr != nil {
			log.Printf("Credit bank failed for %s: %v (continuing anyway)", destBankID, creditErr)
		} else if success {
			log.Printf("Credited %s: +%.2f %s (new balance: %.2f)", destBankID, amount, currency, newBalance)
		}
	}

	return true, "", "", 0
}

// WorkItem represents a message to be processed by the worker pool.
type WorkItem struct {
	Data []byte // Raw ISO 20022 XML message data
}

// readMessages continuously reads messages from Kafka and dispatches them to worker goroutines.
// It handles context cancellation for graceful shutdown and commits offsets after processing.
func readMessages(ctx context.Context, reader *kafka.Reader, workChan chan<- WorkItem) {
	log.Printf("Kafka reader started")

	for {
		select {
		case <-ctx.Done():
			log.Printf("Kafka reader shutting down")
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				log.Printf("Reader: Error reading message: %v", err)
				atomic.AddInt64(&metrics.Errors, 1)
				continue
			}

			atomic.AddInt64(&metrics.MessagesConsumed, 1)

			// Send to worker pool
			select {
			case workChan <- WorkItem{Data: msg.Value}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// processMessages handles message processing in a worker goroutine.
// Each worker performs liquidity checks, validation, and broadcasts results via WebSocket.
// Workers run concurrently to achieve high throughput processing.
func processMessages(ctx context.Context, workChan <-chan WorkItem, workerID int, wg *sync.WaitGroup, results chan<- ValidationResult) {
	defer wg.Done()

	log.Printf("Worker %d started", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d shutting down", workerID)
			return
		case work, ok := <-workChan:
			if !ok {
				log.Printf("Worker %d: work channel closed", workerID)
				return
			}

			// First, check liquidity before XML validation
			liquidityApproved, liquidityErrorCode, liquidityErrorMsg, _ := checkLiquidity(work.Data)

			if !liquidityApproved {
				atomic.AddInt64(&metrics.MessagesInvalid, 1)

				// Send rejection result
				result := ValidationResult{
					Valid:     false,
					ErrorCode: liquidityErrorCode,
					ErrorMsg:  liquidityErrorMsg,
					Timestamp: time.Now(),
				}

				// Broadcast transaction to WebSocket clients
				broadcastTransactionToWS(result, work.Data)

				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
				continue
			}

			// Validate the transaction
			result := validateTransaction(work.Data)

			if result.Valid {
				atomic.AddInt64(&metrics.MessagesValid, 1)
			} else {
				atomic.AddInt64(&metrics.MessagesInvalid, 1)
			}

			// Broadcast transaction to WebSocket clients
			broadcastTransactionToWS(result, work.Data)

			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

// broadcastTransactionToWS broadcasts transaction processing results to WebSocket clients.
// It extracts key transaction details from the XML and sends real-time updates to the dashboard.
func broadcastTransactionToWS(result ValidationResult, xmlData []byte) {
	// Parse XML to extract transaction details
	var doc Document
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return
	}

	if len(doc.FIToFICstmrCdtTrf.CdtTrfTxInf) == 0 {
		return
	}

	tx := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0]

	// Parse amount
	amount := 0.0
	if tx.IntrBkSttlmAmt.Value != "" {
		fmt.Sscanf(tx.IntrBkSttlmAmt.Value, "%f", &amount)
	}

	// Extract country codes from PostalAddr.Ctry with fallback to BIC extraction
	dbtrBIC := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].DbtrAgt.FinInstnId.BICFI
	cdtrBIC := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].CdtrAgt.FinInstnId.BICFI

	sourceCountry := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].Dbtr.PstlAdr.Ctry
	if sourceCountry == "" {
		sourceCountry = extractCountryFromBIC(dbtrBIC)
	}

	destCountry := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].Cdtr.PstlAdr.Ctry
	if destCountry == "" {
		destCountry = extractCountryFromBIC(cdtrBIC)
	}

	// Extract source bank name from Dbtr.Nm
	sourceBankName := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].Dbtr.Nm

	txMsg := TransactionMessage{
		ID:            fmt.Sprintf("tx-%d", time.Now().UnixNano()),
		MsgID:         doc.FIToFICstmrCdtTrf.GrpHdr.MsgId,
		Source:        sourceBankName,
		SourceCountry: sourceCountry,
		Destination:   tx.Cdtr.Nm,
		DestCountry:   destCountry,
		Amount:        amount,
		Currency:      tx.IntrBkSttlmAmt.Ccy,
		Status:        mapStatus(result.Valid),
		ErrorCode:     result.ErrorCode,
		ErrorMsg:      result.ErrorMsg,
		Timestamp:     result.Timestamp,
		XML:           string(xmlData),
		Latency:       result.Latency.Milliseconds(),
	}

	BroadcastTransaction(txMsg)
}

// extractCountryFromBIC extracts the ISO 3166-1 alpha-2 country code from a BIC.
// BICs have the format: BBBBCCLLBBB where positions 4-6 contain the country code.
func extractCountryFromBIC(bic string) string {
	if len(bic) >= 6 {
		return bic[4:6]
	}
	return "XX"
}

// mapStatus converts a boolean validation result to a human-readable status string.
func mapStatus(valid bool) string {
	if valid {
		return "approved"
	}
	return "rejected"
}

// broadcastMetricsPeriodically broadcasts consumer metrics to WebSocket clients every second.
// Includes message counts, throughput, and error statistics for real-time dashboard updates.
func broadcastMetricsPeriodically(hub *WebSocketHub) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			consumed := atomic.LoadInt64(&metrics.MessagesConsumed)
			valid := atomic.LoadInt64(&metrics.MessagesValid)

			totalRate := float64(consumed) / time.Since(metrics.StartTime).Seconds()
			successRate := 0.0
			if consumed > 0 {
				successRate = float64(valid) / float64(consumed) * 100
			}

			metricsMsg := MetricsMessage{
				TotalProcessed:    consumed,
				MessagesPerSec:    totalRate,
				SuccessRate:       successRate,
				ActiveConnections: len(hub.clients),
			}

			BroadcastMetrics(metricsMsg)
		}
	}
}

// broadcastBalancesPeriodically sends bank balances to all connected clients every 5 minutes.
// Performs an initial broadcast after startup to ensure dashboard has current data.
func broadcastBalancesPeriodically(hub *WebSocketHub) {
	// Use a ticker for 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Initial broadcast after a short delay to ensure services are up
	time.Sleep(5 * time.Second)
	broadcastBalances(hub)

	for {
		select {
		case <-ticker.C:
			broadcastBalances(hub)
		}
	}
}

// broadcastBalances retrieves current bank balances from the liquidity service
// and broadcasts them to all connected WebSocket clients for dashboard display.
func broadcastBalances(hub *WebSocketHub) {
	if liquidityClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	balances, err := liquidityClient.GetBalances(ctx)
	if err != nil {
		log.Printf("[WebSocket] Failed to fetch balances for broadcast: %v", err)
		return
	}

	var frontendBalances []BankBalanceMessage
	bicMapMutex.RLock()
	for _, b := range balances {
		name := b.BankId
		bic := b.BankId
		if info, ok := bankInfoMap[b.BankId]; ok {
			name = info.Name
			bic = info.BIC
		}
		frontendBalances = append(frontendBalances, BankBalanceMessage{
			BankName: name,
			BIC:      bic,
			Balance:  b.Balance,
			Currency: b.Currency,
		})
	}
	bicMapMutex.RUnlock()

	log.Printf("[WebSocket] Broadcasting %d bank balances", len(frontendBalances))
	BroadcastBalances(frontendBalances)
}

// getElementValue extracts an XML element value from raw XML data.
// Note: This is a simplified implementation for demo purposes.
// In production, proper XML parsing should be used.
func getElementValue(data []byte, element string) string {
	// Simple string search for demo purposes
	// In production, use proper XML parsing
	str := string(data)
	// This is a simplified approach - for production use proper XML traversal
	return str // Return full XML for logging
}

// logMetrics periodically logs consumer performance metrics every 10 seconds.
// Tracks throughput, error rates, and processing statistics for monitoring.
func logMetrics(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			consumed := atomic.LoadInt64(&metrics.MessagesConsumed)
			valid := atomic.LoadInt64(&metrics.MessagesValid)
			invalid := atomic.LoadInt64(&metrics.MessagesInvalid)
			errors := atomic.LoadInt64(&metrics.Errors)

			duration := now.Sub(metrics.LastLogTime).Seconds()
			msgRate := float64(consumed-metrics.LastMessageCount) / duration
			totalDuration := now.Sub(metrics.StartTime).Seconds()
			totalRate := float64(consumed) / totalDuration

			log.Printf("=== METRICS ===")
			log.Printf("Total Consumed: %d | Valid: %d | Invalid: %d | Errors: %d",
				consumed, valid, invalid, errors)
			log.Printf("Rate (last 10s): %.2f msg/sec | Total Rate: %.2f msg/sec",
				msgRate, totalRate)
			log.Printf("Success Rate: %.2f%%", float64(valid)/float64(consumed)*100)
			log.Printf("Uptime: %v", totalDuration)
			log.Printf("===============")

			metrics.LastLogTime = now
			metrics.LastMessageCount = consumed
		}
	}
}

// main is the entry point for the Nexus-Lite consumer service.
// It initializes Kafka connectivity, starts the worker pool, WebSocket server,
// and begins processing ISO 20022 messages with liquidity validation.
func main() {
	// Initialize metrics
	metrics = Metrics{
		StartTime:   time.Now(),
		LastLogTime: time.Now(),
	}

	// Parse flags
	brokerAddr := flag.String("broker", "localhost:9092", "Kafka broker address")
	liquidityAddr := flag.String("liquidity", "localhost:50051", "Liquidity service address")
	wsAddr := flag.String("ws", ":8080", "WebSocket server address")
	numWorkers := flag.Int("workers", 5, "Number of concurrent workers")
	configPath := flag.String("config", "../config/network.json", "Path to network configuration")
	flag.Parse()

	// Load configuration
	if err := LoadBICMapping(*configPath); err != nil {
		log.Printf("Warning: Failed to load BIC mapping from %s: %v", *configPath, err)
	} else {
		log.Printf("Loaded BIC mappings from %s", *configPath)
	}

	// Initialize liquidity client
	var err error
	liquidityClient, err = NewLiquidityClient(*liquidityAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to liquidity service: %v", err)
		log.Printf("Continuing without liquidity checks...")
		liquidityClient = nil
	} else {
		log.Printf("Connected to liquidity service at %s", *liquidityAddr)
		defer liquidityClient.Close()
	}

	topic := "nexus-transactions"

	// Configure Kafka reader without consumer group for better control
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{*brokerAddr},
		Topic:     topic,
		Partition: 0,                      // Read from partition 0
		MinBytes:  1,                      // Don't wait for minimum bytes
		MaxBytes:  10e6,                   // 10MB
		MaxWait:   100 * time.Millisecond, // Don't wait too long
	})
	// Start from end of topic
	reader.SetOffset(kafka.LastOffset)
	defer reader.Close()

	log.Printf("Starting PayNet Switch Consumer (Phase 4 - Command Center Dashboard)")
	log.Printf("Kafka Broker: %s", *brokerAddr)
	log.Printf("Topic: %s", topic)
	log.Printf("Partition: 0")
	log.Printf("Workers: %d", *numWorkers)
	if liquidityClient != nil {
		log.Printf("Liquidity Service: %s", *liquidityAddr)
	} else {
		log.Printf("Liquidity Service: Disabled (connection failed)")
	}
	log.Printf("WebSocket Server: %s", *wsAddr)
	log.Printf("Press Ctrl+C to stop...")

	// Setup graceful shutdown and context first
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for dependencies to be ready
	if err := waitForKafka(*brokerAddr, 15); err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}

	if err := waitForLiquidityService(*liquidityAddr, 15); err != nil {
		log.Printf("Warning: Liquidity Service not available: %v", err)
		atomic.StoreInt32(&liquidityReady, 0)
	} else {
		atomic.StoreInt32(&liquidityReady, 1)
	}

	// Start dependency monitoring
	go monitorDependencies(ctx, *brokerAddr, *liquidityAddr)

	// Start WebSocket server
	hub := StartWebSocketServer(*wsAddr)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received...")
		cancel()
	}()

	// Start metrics logger
	go logMetrics(ctx)

	// Start metrics broadcaster for WebSocket
	go broadcastMetricsPeriodically(hub)

	// Start balances broadcaster for WebSocket (every 5 minutes)
	go broadcastBalancesPeriodically(hub)

	// Start worker pool with proper pattern
	var wg sync.WaitGroup
	results := make(chan ValidationResult, 1000) // Larger buffer
	workChan := make(chan WorkItem, 1000)        // Buffered channel for work items

	// Start single Kafka reader goroutine
	go readMessages(ctx, reader, workChan)

	// Start worker pool to process messages
	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		go processMessages(ctx, workChan, i+1, &wg, results)
	}

	// Drain results channel to prevent blocking
	go func() {
		for range results {
			// Results are already processed, just drain to prevent blocking
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	close(workChan)

	// Wait for all workers to finish
	wg.Wait()
	close(results)

	// Final metrics
	duration := time.Since(metrics.StartTime)
	consumed := atomic.LoadInt64(&metrics.MessagesConsumed)
	valid := atomic.LoadInt64(&metrics.MessagesValid)
	invalid := atomic.LoadInt64(&metrics.MessagesInvalid)
	errors := atomic.LoadInt64(&metrics.Errors)

	throughput := float64(consumed) / duration.Seconds()

	log.Printf("Consumer stopped. Duration: %v", duration)
	log.Printf("Total Messages: %d | Valid: %d | Invalid: %d | Errors: %d",
		consumed, valid, invalid, errors)
	log.Printf("Average Throughput: %.2f msg/sec", throughput)
	if consumed > 0 {
		log.Printf("Success Rate: %.2f%%", float64(valid)/float64(consumed)*100)
	}
}
