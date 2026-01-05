// Package main implements the ISO 20022 transaction producer for Nexus-Lite.
// This service generates compliant pacs.008.001.08 XML messages and publishes
// them to Kafka for processing by the PayNet Switch consumer.
package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Configuration Structures

// Bank represents a financial institution in the ASEAN network.
// It contains configuration details for participating banks including
// their BIC codes, currencies, and roles in the settlement network.
type Bank struct {
	ID             string   `json:"id"`                        // Unique identifier for the bank
	Name           string   `json:"name"`                      // Human-readable bank name
	BIC            string   `json:"bic"`                       // Bank Identifier Code (11 characters)
	Country        string   `json:"country"`                   // ISO 3166-1 alpha-2 country code
	Currency       string   `json:"currency"`                  // ISO 4217 currency code
	InitialBalance float64  `json:"initial_balance,omitempty"` // Starting balance for liquidity checks
	Roles          []string `json:"role"`                      // Roles: "source", "dest", or both
}

// NetworkConfig holds the complete network configuration loaded from JSON.
type NetworkConfig struct {
	Banks []Bank `json:"banks"` // List of all banks in the network
}

// Global bank list - all banks can be source or destination
var allBanks []Bank

// ISO 20022 pacs.008.001.08 - Full XSD Compliance Structure

// Document represents the root element of an ISO 20022 pacs.008 message.
// It contains the XML namespace declarations and the credit transfer message.
type Document struct {
	XMLName           xml.Name       `xml:"Document"`
	Xmlns             string         `xml:"xmlns,attr"`     // ISO 20022 namespace
	XmlnsXsi          string         `xml:"xmlns:xsi,attr"` // XML Schema Instance namespace
	FIToFICstmrCdtTrf CreditTransfer `xml:"FIToFICstmrCdtTrf"`
}

// CreditTransfer represents the FIToFICstmrCdtTrf element containing
// group header and transaction information.
type CreditTransfer struct {
	GrpHdr      GroupHeader       `xml:"GrpHdr"`      // Group header with message metadata
	CdtTrfTxInf []TransactionInfo `xml:"CdtTrfTxInf"` // Array of individual transactions
}

type GroupHeader struct {
	MsgId    string         `xml:"MsgId"`    // Unique message identifier
	CreDtTm  time.Time      `xml:"CreDtTm"`  // Message creation timestamp
	NbOfTxs  int            `xml:"NbOfTxs"`  // Number of transactions in this message
	SttlmInf SettlementInfo `xml:"SttlmInf"` // Settlement information
	InstgAgt Agent          `xml:"InstgAgt"` // Instructing agent (source bank)
	InstdAgt Agent          `xml:"InstdAgt"` // Instructed agent (destination bank)
}

// SettlementInfo contains settlement method and clearing system details.
type SettlementInfo struct {
	SttlmMtd string         `xml:"SttlmMtd"` // Settlement method (e.g., "CLRG")
	ClrSys   ClearingSystem `xml:"ClrSys"`   // Clearing system identifier
}

// ClearingSystem identifies the clearing system used for settlement.
type ClearingSystem struct {
	Prtry string `xml:"Prtry"` // Proprietary clearing system code (e.g., "NEXUS")
}

// TransactionInfo contains details for an individual credit transfer transaction.
type TransactionInfo struct {
	PmtId          PaymentID `xml:"PmtId"`          // Payment identifiers
	IntrBkSttlmAmt Amount    `xml:"IntrBkSttlmAmt"` // Interbank settlement amount
	IntrBkSttlmDt  string    `xml:"IntrBkSttlmDt"`  // Settlement date
	ChrgBr         string    `xml:"ChrgBr"`         // Charge bearer (e.g., "SHAR")
	InstgAgt       Agent     `xml:"InstgAgt"`       // Instructing agent for this transaction
	InstdAgt       Agent     `xml:"InstdAgt"`       // Instructed agent for this transaction
	Dbtr           Party     `xml:"Dbtr"`           // Debtor (sending customer)
	DbtrAcct       Account   `xml:"DbtrAcct"`       // Debtor account
	DbtrAgt        Agent     `xml:"DbtrAgt"`        // Debtor agent (source bank)
	CdtrAgt        Agent     `xml:"CdtrAgt"`        // Creditor agent (destination bank)
	Cdtr           Party     `xml:"Cdtr"`           // Creditor (receiving customer)
	CdtrAcct       Account   `xml:"CdtrAcct"`       // Creditor account
}

// PaymentID contains unique identifiers for the payment instruction.
type PaymentID struct {
	InstrId    string `xml:"InstrId"`    // Instruction identifier
	EndToEndId string `xml:"EndToEndId"` // End-to-end identifier
	UETR       string `xml:"UETR"`       // Unique End-to-end Transaction Reference
}

// Party represents a customer (debtor or creditor) in the transaction.
type Party struct {
	Nm      string     `xml:"Nm"`      // Party name
	PstlAdr PostalAddr `xml:"PstlAdr"` // Postal address
}

// PostalAddr contains the party's postal address information.
type PostalAddr struct {
	Ctry string `xml:"Ctry"` // ISO 3166-1 alpha-2 country code
}

// Account represents a bank account with its identifier.
type Account struct {
	Id AccountID `xml:"Id"` // Account identifier
}

// AccountID contains the account identification details.
type AccountID struct {
	Othr OtherID `xml:"Othr"` // Other account identifier
}

// OtherID represents an account identifier with scheme information.
type OtherID struct {
	Id      string `xml:"Id"`      // Account identifier value
	SchmeNm Scheme `xml:"SchmeNm"` // Identification scheme name
}

// Scheme specifies the account identification scheme.
type Scheme struct {
	Prtry string `xml:"Prtry"` // Proprietary scheme name (e.g., "BBAN")
}

// Agent represents a financial institution (bank) in the transaction chain.
type Agent struct {
	FinInstnId FinancialInstitution `xml:"FinInstnId"` // Financial institution identifier
}

// FinancialInstitution contains the BIC code for a bank.
type FinancialInstitution struct {
	BICFI string `xml:"BICFI"` // Bank Identifier Code (11 characters)
}

// Amount represents a monetary amount with currency.
type Amount struct {
	Ccy   string `xml:"Ccy,attr"`  // ISO 4217 currency code
	Value string `xml:",chardata"` // Amount value
}

// Metrics for monitoring
type Metrics struct {
	MessagesProduced int64     // Total number of messages successfully published to Kafka
	Errors           int64     // Total number of errors encountered
	StartTime        time.Time // Service start timestamp for uptime calculation
}

var metrics Metrics

// loadConfig loads bank network configuration from a JSON file.
// It filters banks to include only those with "source" or "dest" roles
// and ensures at least two banks are available for transactions.
func loadConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var config NetworkConfig
	if err := json.Unmarshal(bytes, &config); err != nil {
		return err
	}

	allBanks = []Bank{}

	// Load all banks that can participate in transactions
	for _, bank := range config.Banks {
		hasSourceRole := false
		hasDestRole := false
		for _, role := range bank.Roles {
			if role == "source" {
				hasSourceRole = true
			}
			if role == "dest" {
				hasDestRole = true
			}
		}
		// Bank must be able to send OR receive (preferably both for many-to-many)
		if hasSourceRole || hasDestRole {
			allBanks = append(allBanks, bank)
		}
	}

	if len(allBanks) < 2 {
		return fmt.Errorf("configuration must have at least two banks for transactions")
	}

	return nil
}

// generateTransaction creates a random ISO 20022 pacs.008.001.08 credit transfer message.
// It selects random source and destination banks, generates realistic customer names,
// and constructs a fully compliant XML message with proper namespaces and BIC codes.
func generateTransaction(msgID int) ([]byte, error) {
	if len(allBanks) < 2 {
		return nil, fmt.Errorf("need at least 2 banks configured")
	}

	// Select random source bank
	sourceIdx := rand.Intn(len(allBanks))
	sourceBank := allBanks[sourceIdx]

	// Select random destination bank (must be different from source)
	destIdx := rand.Intn(len(allBanks))
	for destIdx == sourceIdx {
		destIdx = rand.Intn(len(allBanks))
	}
	destBank := allBanks[destIdx]

	amount := rand.Float64()*9999 + 1 // Amount between 1 and 10000

	// Use source bank's currency (sender pays in their currency)
	currency := sourceBank.Currency

	// Adjust amount for currency - IDR and VND use larger amounts due to lower denominations
	if currency == "IDR" || currency == "VND" {
		amount = amount * 1000
	}

	// Normalize BICs to 11 characters (add XXX if 8 chars) for ISO 20022 compliance
	sourceBIC := normalizeBIC(sourceBank.BIC)
	destBIC := normalizeBIC(destBank.BIC)

	// Generate customer names (underlying customers, not banks) - ISO 20022 distinguishes parties from agents
	dbtrName := fmt.Sprintf("%s Treasury Services", sourceBank.Name)
	cdtrName := fmt.Sprintf("%s Corporate Client", destBank.Name)

	doc := Document{
		Xmlns:    "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
		FIToFICstmrCdtTrf: CreditTransfer{
			GrpHdr: GroupHeader{
				MsgId:   fmt.Sprintf("PAYNET-NEXUS-%d-%d", time.Now().Unix(), msgID),
				CreDtTm: time.Now(),
				NbOfTxs: 1,
				SttlmInf: SettlementInfo{
					SttlmMtd: "CLRG",
					ClrSys:   ClearingSystem{Prtry: "NEXUS"},
				},
				InstgAgt: Agent{FinInstnId: FinancialInstitution{BICFI: sourceBIC}},
				InstdAgt: Agent{FinInstnId: FinancialInstitution{BICFI: destBIC}},
			},
			CdtTrfTxInf: []TransactionInfo{
				{
					PmtId: PaymentID{
						InstrId:    fmt.Sprintf("TXN-%d-%d", time.Now().Unix(), msgID),
						EndToEndId: fmt.Sprintf("E2E-%d-%d", time.Now().Unix(), msgID),
						UETR:       uuid.New().String(),
					},
					IntrBkSttlmAmt: Amount{
						Ccy:   currency,
						Value: fmt.Sprintf("%.2f", amount),
					},
					IntrBkSttlmDt: time.Now().Format("2006-01-02"),
					ChrgBr:        "SHAR",
					InstgAgt: Agent{
						FinInstnId: FinancialInstitution{BICFI: sourceBIC},
					},
					InstdAgt: Agent{
						FinInstnId: FinancialInstitution{BICFI: destBIC},
					},
					Dbtr: Party{
						Nm:      dbtrName,
						PstlAdr: PostalAddr{Ctry: sourceBank.Country},
					},
					DbtrAcct: Account{
						Id: AccountID{
							Othr: OtherID{
								Id:      fmt.Sprintf("%s%08d", sourceBank.Country, rand.Intn(99999999)),
								SchmeNm: Scheme{Prtry: "BBAN"},
							},
						},
					},
					DbtrAgt: Agent{
						FinInstnId: FinancialInstitution{BICFI: sourceBIC},
					},
					CdtrAgt: Agent{
						FinInstnId: FinancialInstitution{BICFI: destBIC},
					},
					Cdtr: Party{
						Nm:      cdtrName,
						PstlAdr: PostalAddr{Ctry: destBank.Country},
					},
					CdtrAcct: Account{
						Id: AccountID{
							Othr: OtherID{
								Id:      fmt.Sprintf("%s%08d", destBank.Country, rand.Intn(99999999)),
								SchmeNm: Scheme{Prtry: "BBAN"},
							},
						},
					},
				},
			},
		},
	}

	return xml.MarshalIndent(doc, "", "  ")
}

// normalizeBIC ensures BIC is 11 characters (adds XXX if 8 chars) for ISO 20022 compliance.
// SWIFT BICs are either 8 or 11 characters; ISO 20022 requires 11 characters.
func normalizeBIC(bic string) string {
	if len(bic) == 8 {
		return bic + "XXX"
	}
	return bic
}

// produceMessages continuously generates and publishes ISO 20022 messages to Kafka.
// It runs a ticker at the specified messages per second rate and handles graceful shutdown.
// The function also periodically logs batch statistics and monitors for context cancellation.
func produceMessages(ctx context.Context, writer *kafka.Writer, messagesPerSecond int) {
	ticker := time.NewTicker(time.Second / time.Duration(messagesPerSecond)) // Generate messages at target rate
	defer ticker.Stop()

	batchTicker := time.NewTicker(10 * time.Second) // Log batch stats every 10 seconds
	defer batchTicker.Stop()

	batchCount := 0
	msgID := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("Producer shutting down. Total messages: %d", atomic.LoadInt64(&metrics.MessagesProduced))
			return
		case <-batchTicker.C:
			log.Printf("Batch completed. Messages in batch: %d, Total: %d, Errors: %d",
				batchCount, atomic.LoadInt64(&metrics.MessagesProduced), atomic.LoadInt64(&metrics.Errors))
			batchCount = 0
		case <-ticker.C:
			data, err := generateTransaction(msgID)
			if err != nil {
				log.Printf("Error generating transaction: %v", err)
				atomic.AddInt64(&metrics.Errors, 1)
				continue
			}

			message := kafka.Message{
				Key:   []byte(fmt.Sprintf("txn-%d", msgID)), // Partition key for ordering
				Value: data,
				Time:  time.Now(),
			}

			err = writer.WriteMessages(ctx, message)
			if err != nil {
				log.Printf("Error writing message to Kafka: %v", err)
				atomic.AddInt64(&metrics.Errors, 1)
			} else {
				atomic.AddInt64(&metrics.MessagesProduced, 1)
				batchCount++
			}
			msgID++
		}
	}
}

// main is the entry point for the Nexus-Lite producer service.
// It initializes configuration, sets up Kafka connectivity, starts health monitoring,
// and begins producing ISO 20022 messages at the configured rate.
func main() {
	// Initialize metrics
	metrics = Metrics{
		StartTime: time.Now(),
	}

	// Parse command line flags
	brokerAddr := flag.String("broker", "localhost:9092", "Kafka broker address")
	targetTPS := flag.Int("tps", 1000, "Target transactions per second")
	configPath := flag.String("config", "../config/network.json", "Path to network configuration")
	healthAddr := flag.String("health", ":8081", "Health check server address")
	flag.Parse()

	// Load bank network configuration
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", *configPath, err)
	}
	atomic.StoreInt32(&configLoaded, 1)
	log.Printf("Loaded %d banks for many-to-many transactions", len(allBanks))

	topic := "nexus-transactions"

	// Configure Kafka writer with optimized settings for high throughput
	writer := &kafka.Writer{
		Addr:         kafka.TCP(*brokerAddr),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},   // Distribute load evenly
		Compression:  kafka.Snappy,          // Fast compression
		BatchSize:    100,                   // Batch messages for efficiency
		BatchTimeout: 10 * time.Millisecond, // Low latency batching
		Async:        true,                  // Non-blocking writes
	}
	defer writer.Close()

	// Create topic if it doesn't exist (idempotent operation)
	conn, err := kafka.Dial("tcp", *brokerAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		log.Printf("Warning: Could not get controller: %v", err)
	} else {
		controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
		if err == nil {
			defer controllerConn.Close()

			topicConfigs := []kafka.TopicConfig{
				{
					Topic:             topic,
					NumPartitions:     3,
					ReplicationFactor: 1,
				},
			}

			err = controllerConn.CreateTopics(topicConfigs...)
			if err != nil && err != kafka.TopicAlreadyExists {
				log.Printf("Warning: Could not create topic: %v", err)
			}
		}
	}

	// Start health check server
	startHealthServer(*healthAddr)

	// Initial Kafka health check
	if err := checkKafkaHealth(*brokerAddr); err != nil {
		log.Printf("Warning: Initial Kafka health check failed: %v", err)
		atomic.StoreInt32(&kafkaHealthy, 0)
	} else {
		atomic.StoreInt32(&kafkaHealthy, 1)
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Kafka health monitoring
	go monitorKafkaHealth(ctx, *brokerAddr)

	log.Printf("Starting Bank A Producer")
	log.Printf("Kafka Broker: %s", *brokerAddr)
	log.Printf("Topic: %s", topic)
	log.Printf("Target Throughput: %d messages/second", *targetTPS)
	log.Printf("Press Ctrl+C to stop...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received...")
		cancel()
	}()

	// Start producing messages
	produceMessages(ctx, writer, *targetTPS)

	// Final metrics
	duration := time.Since(metrics.StartTime)
	throughput := float64(atomic.LoadInt64(&metrics.MessagesProduced)) / duration.Seconds()
	log.Printf("Producer stopped. Duration: %v, Total Messages: %d, Throughput: %.2f msg/sec, Errors: %d",
		duration, atomic.LoadInt64(&metrics.MessagesProduced), throughput, atomic.LoadInt64(&metrics.Errors))
}
