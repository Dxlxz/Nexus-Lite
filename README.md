# Nexus-Lite: Phase 4 - Command Center Dashboard

## Overview
Nexus-Lite is a cloud-native, ISO 20022 cross-border settlement engine for PayNet's "Next50" George Town Accord initiative.

## Phase 1: ISO 20022 Parser
This phase implements the core ISO 20022 (pacs.008) parser that transforms legacy transaction formats into compliant ISO 20022 XML messages.

### Phase 1 Implementation Details

The parser uses Go's built-in `encoding/xml` package to generate ISO 20022 pacs.008 compliant XML messages. The structure includes:

- **Document**: Root element with ISO 20022 namespace
- **CreditTransfer**: FIToFICstmrCdtTrf message structure
- **GroupHeader**: Message-level information (MsgId, CreDtTm, NbOfTxs)
- **TransactionInfo**: Individual transaction details (PmtId, Amount)
- **Amount**: Currency and value representation

### Running the Phase 1 Parser

```bash
go run main.go
```

### Phase 1 Sample Output
The parser generates valid ISO 20022 XML with the following structure:
- Message ID with timestamp (PAYNET-NEXUS-{unix-timestamp})
- Creation timestamp in ISO 8601 format
- Transaction count
- Payment instruction ID
- Settlement amount with currency code (MYR)

### Phase 1 Technical Specifications
- **Standard**: ISO 20022 pacs.008.001.08
- **Namespace**: `urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08`
- **Currency**: Malaysian Ringgit (MYR)
- **Dependencies**: Go standard library only (encoding/xml, fmt, time)

### Phase 1 Status
✅ Complete - ISO 20022 parser implementation ready for testing and integration.

---

## Phase 2: Kafka Streaming Pipeline

This phase implements a high-throughput Kafka streaming pipeline that simulates the PayNet Switch handling real-time transaction bursts (e.g., 11.11 sale traffic).

### Architecture Overview

```
┌─────────────┐         ┌──────────────┐         ┌──────────────────┐
│   Bank A    │         │    Kafka     │         │  PayNet Switch   │
│  (Producer) │────────▶│   Cluster    │────────▶│   (Consumer)     │
│             │         │              │         │                  │
│ 1,000 msg/s │         │ Topic:       │         │ - XML Validation │
│             │         │ nexus-trans  │         │ - Error Handling │
└─────────────┘         └──────────────┘         └──────────────────┘
                                ▲
                                │
                        ┌───────┴────────┐
                        │   Zookeeper   │
                        │   (Port 2181)  │
                        └────────────────┘
```

### Project Structure
```
nexus-lite/
├── docker-compose.yml          # Kafka + Zookeeper configuration
├── go.mod                      # Go module with Kafka dependencies
├── go.sum                      # Dependency checksums
├── main.go                     # Phase 1: ISO 20022 parser
├── producer/
│   └── main.go                 # Bank A - Transaction Producer
├── consumer/
│   └── main.go                 # PayNet Switch - Transaction Consumer
└── README.md                   # This file
```

### Phase 2 Components

#### 1. Docker Compose Configuration
The [`docker-compose.yml`](nexus-lite/docker-compose.yml) file defines:
- **Zookeeper** service (port 2181) - Kafka coordination
- **Kafka** broker service (port 9092) - Message streaming
- Volume persistence for data durability
- Proper environment variables for Kafka configuration

#### 2. Producer Service (Bank A)
The [`producer/main.go`](nexus-lite/producer/main.go) implements:
- ISO 20022 pacs.008 XML message generation
- Publishing to Kafka topic: `nexus-transactions`
- Target throughput: 1,000 messages/second
- Burst handling: Capable of 10,000+ messages in bursts
- Transaction metadata (timestamp, source bank, destination bank)
- Graceful shutdown handling
- Metrics tracking (messages produced, errors)

**Supported Source Banks:**
- Maybank, CIMB, Public Bank, RHB Bank, Hong Leong Bank, UOB Malaysia, OCBC Malaysia, Bank Islam, AmBank

**Supported Destination Banks:**
- Singapore: DBS Bank, UOB, OCBC
- Thailand: Bangkok Bank, Kasikornbank, Krungthai Bank
- Indonesia: Bank Mandiri, BCA, BNI
- Philippines: BPI, Metrobank, BDO
- Vietnam: Vietcombank, Vietinbank, Techcombank

#### 3. Consumer Service (PayNet Switch)
The [`consumer/main.go`](nexus-lite/consumer/main.go) implements:
- Subscription to `nexus-transactions` topic
- ISO 20022 XML structure validation
- Comprehensive validation rules:
  - XML parsing validation
  - Message ID presence
  - Transaction count validation
  - BIC format validation (8 or 11 characters)
  - Payment ID validation
  - Currency and amount validation
- Logging of successful/failed transactions
- Worker pool architecture (5 concurrent workers)
- Resilience: Continues processing on individual message failures
- Metrics tracking (consumed, valid, invalid, errors, throughput)
- Graceful shutdown handling

### Setup Instructions

#### Prerequisites
- Docker and Docker Compose installed
- Go 1.21 or higher installed
- Port 2181 and 9092 available on localhost

#### Step 1: Start Kafka Infrastructure
```bash
cd nexus-lite
docker-compose up -d
```

This will start Zookeeper and Kafka services. Wait for Kafka to be fully initialized (usually 30-60 seconds).

#### Step 2: Install Go Dependencies
```bash
go mod download
```

#### Step 3: Run the Producer (Bank A)
In a new terminal:
```bash
cd nexus-lite/producer
go run main.go
```

The producer will start generating and publishing transactions at 1,000 messages/second.

**Producer Output:**
```
2024/01/02 14:50:00 Starting Bank A Producer
2024/01/02 14:50:00 Kafka Broker: localhost:9092
2024/01/02 14:50:00 Topic: nexus-transactions
2024/01/02 14:50:00 Target Throughput: 1,000 messages/second
2024/01/02 14:50:00 Press Ctrl+C to stop...
2024/01/02 14:50:10 Batch completed. Messages in batch: 10000, Total: 10000, Errors: 0
```

#### Step 4: Run the Consumer (PayNet Switch)
In another new terminal:
```bash
cd nexus-lite/consumer
go run main.go
```

The consumer will start processing transactions with validation.

**Consumer Output:**
```
2024/01/02 14:50:05 Starting PayNet Switch Consumer
2024/01/02 14:50:05 Kafka Broker: localhost:9092
2024/01/02 14:50:05 Topic: nexus-transactions
2024/01/02 14:50:05 Consumer Group: paynet-switch-consumer
2024/01/02 14:50:05 Workers: 5
2024/01/02 14:50:05 Press Ctrl+C to stop...
2024/01/02 14:50:10 === METRICS ===
2024/01/02 14:50:10 Total Consumed: 5000 | Valid: 5000 | Invalid: 0 | Errors: 0
2024/01/02 14:50:10 Rate (last 10s): 500.00 msg/sec | Total Rate: 500.00 msg/sec
2024/01/02 14:50:10 Success Rate: 100.00%
2024/01/02 14:50:10 Uptime: 10.00s
2024/01/02 14:50:10 ===============
```

### Advanced Usage

#### Custom Kafka Broker Address
To connect to a different Kafka broker:
```bash
go run main.go kafka-broker-host:9092
```

#### Scaling the Consumer
To increase consumer throughput, modify the `numWorkers` variable in [`consumer/main.go`](nexus-lite/consumer/main.go):
```go
numWorkers := 10 // Increase from 5 to 10
```

#### Adjust Producer Throughput
To change the production rate, modify the `produceMessages` call in [`producer/main.go`](nexus-lite/producer/main.go):
```go
produceMessages(ctx, writer, 2000) // Increase to 2,000 msg/sec
```

### Validation Error Codes

The consumer validates transactions and returns specific error codes:

| Error Code | Description |
|------------|-------------|
| `XML_PARSE_ERROR` | Failed to parse XML message |
| `MISSING_MSG_ID` | Message ID is missing |
| `INVALID_TX_COUNT` | Transaction count is invalid |
| `INVALID_DBTR_BIC` | Debtor BIC format is invalid |
| `INVALID_CDTR_BIC` | Creditor BIC format is invalid |
| `NO_TRANSACTIONS` | No transactions found in message |
| `MISSING_PMT_ID_TX{n}` | Payment ID missing for transaction n |
| `MISSING_CURRENCY_TX{n}` | Currency missing for transaction n |
| `MISSING_AMOUNT_TX{n}` | Amount missing for transaction n |

### Performance Metrics

The system tracks the following metrics:

**Producer Metrics:**
- Messages Produced: Total messages sent to Kafka
- Errors: Number of send failures
- Throughput: Messages per second

**Consumer Metrics:**
- Messages Consumed: Total messages received from Kafka
- Messages Valid: Successfully validated transactions
- Messages Invalid: Failed validation transactions
- Errors: Processing errors
- Throughput: Messages processed per second
- Success Rate: Percentage of valid messages

### Stopping the Pipeline

To stop the services gracefully:

1. Press `Ctrl+C` in the producer terminal
2. Press `Ctrl+C` in the consumer terminal
3. Stop Docker containers:
```bash
docker-compose down
```

### Troubleshooting

#### Kafka Connection Issues
If you see connection errors:
1. Verify Kafka is running: `docker-compose ps`
2. Check Kafka logs: `docker-compose logs kafka`
3. Ensure ports 2181 and 9092 are available

#### Consumer Lag
If the consumer is falling behind:
1. Increase the number of workers in [`consumer/main.go`](nexus-lite/consumer/main.go)
2. Run multiple consumer instances in different terminals
3. Add more Kafka partitions

#### High Memory Usage
If you experience memory issues:
1. Reduce the Kafka reader batch size
2. Limit the number of concurrent workers
3. Increase the consumer commit interval

### Phase 2 Technical Specifications

**Producer Specifications:**
- Transaction format: ISO 20022 pacs.008 XML
- Target throughput: 1,000 messages/second
- Burst handling: 10,000+ messages in bursts
- Compression: Snappy
- Async writes enabled

**Consumer Specifications:**
- Validation: XML schema validation
- Error handling: Reject malformed messages with error codes
- Logging: Track message processing rate
- Resilience: Continue processing on individual message failures
- Worker pool: 5 concurrent workers (configurable)
- Consumer group: `paynet-switch-consumer`

**Kafka Configuration:**
- Topic: `nexus-transactions`
- Partitions: 3
- Replication Factor: 1
- Retention: 24 hours

### Phase 2 Status
✅ Complete - Kafka streaming pipeline implementation ready for testing and integration.

---

## Phase 3: Liquidity Check Integration

This phase implements an intelligent liquidity prediction service using Python (Scikit-Learn) exposed via gRPC, integrated with the Go payment switch to auto-reject transactions when a bank has insufficient funds.

### Architecture Overview

```
┌─────────────┐         ┌──────────────┐         ┌──────────────────┐         ┌──────────────────┐
│   Bank A    │         │    Kafka     │         │  PayNet Switch   │         │  Liquidity       │
│  (Producer) │────────▶│   Cluster    │────────▶│   (Consumer)     │────────▶│  Service         │
│             │         │              │         │                  │         │  (Python/gRPC)   │
│ 1,000 msg/s │         │ Topic:       │         │ - XML Validation │         │  - Balance Check │
│             │         │ nexus-trans  │         │ - Error Handling │         │  - AM04 Reject   │
└─────────────┘         └──────────────┘         │ - Liquidity Check │         │  - AC04 Reject   │
                                 ▲                 └──────────────────┘         └──────────────────┘
                                 │
                         ┌───────┴────────┐
                         │   Zookeeper   │
                         │   (Port 2181)  │
                         └────────────────┘
```

### Phase 3 Components

#### 1. gRPC Protocol Definition
The [`proto/liquidity.proto`](nexus-lite/proto/liquidity.proto) file defines:
- **LiquidityCheckService**: gRPC service for liquidity checks
- **LiquidityCheckRequest**: bank_id, transaction_amount, currency
- **LiquidityCheckResponse**: approved, available_balance, error_code, error_message

**Error Codes (ISO 20022 Standard):**
| Code | Description |
|------|-------------|
| `AC04` | Closed Account (bank not found) |
| `AM04` | Insufficient Funds |
| `OK` | Transaction Approved |

#### 2. Python Liquidity Service
The [`liquidity-service/`](nexus-lite/liquidity-service/) directory contains:

**Files:**
- [`requirements.txt`](nexus-lite/liquidity-service/requirements.txt) - Python dependencies
- [`model.py`](nexus-lite/liquidity-service/model.py) - Liquidity prediction model
- [`server.py`](nexus-lite/liquidity-service/server.py) - gRPC server implementation
- [`main.py`](nexus-lite/liquidity-service/main.py) - Entry point
- [`Dockerfile`](nexus-lite/liquidity-service/Dockerfile) - Docker configuration

**Model Features:**
- In-memory bank balances for Malaysian banks:
  - MAYBANK: 5,000,000 MYR
  - CIMB: 3,500,000 MYR
  - RHB: 2,800,000 MYR
  - PUBLIC_BANK: 4,200,000 MYR
  - AMBANK: 1,900,000 MYR
  - HSBC: 6,100,000 MYR
  - STANDARD_CHARTERED: 5,500,000 MYR
  - UOB: 4,800,000 MYR
  - OCBC: 5,200,000 MYR
- Transaction history tracking
- Liquidity risk prediction (placeholder for ML model)

#### 3. Go Consumer Integration
The [`consumer/main.go`](nexus-lite/consumer/main.go) has been updated with:
- **Liquidity Client**: gRPC client wrapper ([`liquidity_client.go`](nexus-lite/consumer/liquidity_client.go))
- **BIC Mapping**: Maps BIC codes to bank IDs
- **Pre-validation Check**: Liquidity check before XML validation
- **Auto-rejection**: Skips validation if liquidity check fails
- **Error Logging**: Logs rejection reasons with ISO 20022 error codes

**Integration Flow:**
1. Consumer receives message from Kafka
2. Extracts bank ID, amount, and currency from XML
3. Calls liquidity service via gRPC (100ms timeout)
4. If rejected: Log error, skip validation, continue to next message
5. If approved: Proceed with normal XML validation flow

#### 4. Test Cases
The [`test/liquidity_test.go`](nexus-lite/test/liquidity_test.go) file includes:
- Normal transaction (approved)
- Insufficient funds (rejected with AM04)
- Unknown bank (rejected with AC04)
- Multiple transactions draining balance (sequential test)
- Timeout handling
- Invalid amount handling
- Currency variation tests
- Multiple bank tests
- Large transaction tests
- Performance benchmarks

### Setup Instructions

#### Prerequisites
- Docker and Docker Compose installed
- Go 1.21 or higher installed
- Python 3.11+ installed (for local development)
- Port 2181, 9092, and 50051 available on localhost

#### Step 1: Start All Services
```bash
cd nexus-lite
docker-compose up -d
```

This will start:
- Zookeeper (port 2181)
- Kafka (port 9092)
- Liquidity Service (port 50051)

Wait for all services to be fully initialized (usually 30-60 seconds).

#### Step 2: Install Go Dependencies
```bash
go mod download
go mod tidy
```

#### Step 3: Run the Producer (Bank A)
In a new terminal:
```bash
cd nexus-lite/producer
go run main.go
```

The producer will start generating and publishing transactions at 1,000 messages/second.

#### Step 4: Run the Consumer (PayNet Switch)
In another new terminal:
```bash
cd nexus-lite/consumer
go run main.go
```

The consumer will start processing transactions with liquidity checks.

**Consumer Output with Liquidity Check:**
```
2024/01/02 14:50:05 Starting PayNet Switch Consumer (Phase 3 - Liquidity Check Integration)
2024/01/02 14:50:05 Kafka Broker: localhost:9092
2024/01/02 14:50:05 Topic: nexus-transactions
2024/01/02 14:50:05 Consumer Group: paynet-switch-consumer
2024/01/02 14:50:05 Workers: 5
2024/01/02 14:50:05 Liquidity Service: localhost:50051
2024/01/02 14:50:05 Press Ctrl+C to stop...
2024/01/02 14:50:10 Worker 1: [REJECTED] Liquidity Check - ErrorCode: AM04, ErrorMsg: Liquidity check rejected: AM04
2024/01/02 14:50:10 Worker 2: [VALID] MsgId: PAYNET-NEXUS-1704201800-1234, Source: Maybank, Dest: DBS Bank, Amount: MYR 1234.56, Latency: 5ms
```

### Running Tests

#### Run All Tests
```bash
cd nexus-lite
go test ./test/... -v
```

#### Run Specific Test
```bash
cd nexus-lite
go test ./test/... -v -run TestLiquidityCheckService
```

#### Run Benchmark
```bash
cd nexus-lite
go test ./test/... -bench=BenchmarkLiquidityCheck -benchmem
```

### Advanced Usage

#### Running Liquidity Service Locally (for development)
```bash
cd nexus-lite/liquidity-service

# Install dependencies
pip install -r requirements.txt

# Generate protobuf code (if needed)
python -m grpc_tools.protoc -I../proto --python_out=. --grpc_python_out=. ../proto/liquidity.proto

# Run the service
python main.py --port 50051 --workers 10
```

#### Custom Liquidity Service Address
To connect to a different liquidity service:
```bash
cd nexus-lite/consumer
go run main.go localhost:9092 localhost:50051
```

#### Testing Specific Rejection Scenarios

**Test Insufficient Funds (AM04):**
```bash
# Send a large transaction from AMBANK (balance: 1.9M)
# Transaction > 1.9M should be rejected with AM04
```

**Test Unknown Bank (AC04):**
```bash
# Send transaction from unknown bank
# Should be rejected with AC04
```

### Phase 3 Technical Specifications

**gRPC Service Specifications:**
- Port: 50051
- Protocol: HTTP/2
- Serialization: Protocol Buffers
- Timeout: 100ms per request
- Max Workers: 10 (configurable)

**Liquidity Model Specifications:**
- Initial implementation: In-memory balance check
- Future enhancement: Scikit-Learn predictive model
- Response time: < 50ms
- Error codes: ISO 20022 standard (AC04, AM04, OK)

**Integration Points:**
- Consumer calls liquidity check before XML validation
- If rejected: Skip validation, log error, continue to next message
- If approved: Proceed with normal validation flow
- Fail-open behavior: Continue with validation if liquidity service is unavailable

### Error Codes Reference

| Error Code | Description | Scenario |
|------------|-------------|-----------|
| `AC04` | Closed Account | Bank ID not found in system |
| `AM04` | Insufficient Funds | Transaction amount exceeds available balance |
| `OK` | Approved | Sufficient funds for transaction |
| `LIQUIDITY_CHECK_ERROR` | Internal Error | Failed to parse XML or amount |

### Stopping the System

To stop all services gracefully:

1. Press `Ctrl+C` in the producer terminal
2. Press `Ctrl+C` in the consumer terminal
3. Stop Docker containers:
```bash
docker-compose down
```

### Troubleshooting

#### Liquidity Service Connection Issues
If the consumer reports "Failed to connect to liquidity service":
1. Verify liquidity service is running: `docker-compose ps`
2. Check liquidity service logs: `docker-compose logs liquidity-service`
3. Ensure port 50051 is available

#### Transactions Always Approved
If all transactions are approved regardless of amount:
1. Check if liquidity client is connected (look for "Liquidity Service: Disabled" in logs)
2. Verify liquidity service is responding
3. Check for timeout errors in logs

#### High Latency
If liquidity checks are slow:
1. Check network latency between consumer and liquidity service
2. Reduce timeout in [`liquidity_client.go`](nexus-lite/consumer/liquidity_client.go)
3. Increase number of workers in liquidity service

### Phase 3 Status
✅ Complete - Liquidity check integration with gRPC service ready for testing.

---

## Phase 4: Command Center Dashboard

This phase implements a sci-fi themed real-time visualization dashboard that displays cross-border settlement transactions across ASEAN countries with live WebSocket updates.

### Architecture Overview

```
┌─────────────┐         ┌──────────────┐         ┌──────────────────┐         ┌──────────────────┐
│   Bank A    │         │    Kafka     │         │  PayNet Switch   │         │  Command Center  │
│  (Producer) │────────▶│   Cluster    │────────▶│   (Consumer)     │────────▶│   (Dashboard)    │
│             │         │              │         │                  │         │                  │
│ 1,000 msg/s │         │ Topic:       │         │ - XML Validation │         │ - ASEAN Map      │
│             │         │ nexus-trans  │         │ - Liquidity Check │         │ - Real-time Tx    │
└─────────────┘         └──────────────┘         │ - WebSocket       │         │ - ISO 20022 XML   │
                                  ▲                 └──────────────────┘         └──────────────────┘
                                  │
                          ┌───────┴────────┐
                          │   Zookeeper   │
                          │   (Port 2181)  │
                          └────────────────┘
```

### Phase 4 Components

#### 1. Command Center Dashboard
The [`dashboard/`](nexus-lite/dashboard/) directory contains a Next.js application with:

**Technology Stack:**
- Next.js 14 with App Router
- TypeScript for type safety
- Tailwind CSS for styling
- Framer Motion for animations
- Lucide React for icons

**Features:**
- **ASEAN Map Visualization**: Interactive SVG map showing bank nodes across ASEAN countries (Malaysia, Singapore, Thailand, Indonesia, Philippines, Vietnam)
- **Real-Time Transaction Animation**: Animated particles showing transaction flow between countries with color-coded status (green=approved, red=rejected, yellow=pending)
- **ISO 20022 XML Detail View**: Syntax-highlighted XML display with transaction metadata, error codes, and copy-to-clipboard functionality
- **Live Statistics Panel**: Real-time metrics including total transactions processed, messages per second, success rate, active connections, and top 5 bank balances
- **Transaction List**: Recent transactions table with status indicators and click-to-view XML functionality

**Visual Design:**
- Dark sci-fi theme with deep space blue/black background (#0a0e17)
- Neon color palette (cyan #00f0ff, green #00ff88, red #ff0055, yellow #ffcc00)
- Glow effects, scanlines, animated borders, and grid background
- Monospace fonts for data display

#### 2. WebSocket Server
The [`consumer/websocket.go`](nexus-lite/consumer/websocket.go) file implements:

**Features:**
- WebSocket server on port 8080 (configurable)
- Real-time transaction broadcasts
- Metrics updates every second
- Connection management with automatic reconnection
- Health check endpoint

**WebSocket Messages:**
- `transaction`: Broadcasts each processed transaction with full details
- `metrics`: Broadcasts system metrics (total processed, msg/sec, success rate, active connections)
- `status`: Connection status updates

#### 3. Updated Consumer Integration
The [`consumer/main.go`](nexus-lite/consumer/main.go) has been updated with:

**Changes:**
- WebSocket server startup on port 8080
- Transaction broadcasting to WebSocket clients
- Periodic metrics broadcasting (every second)
- Country code extraction from BIC codes
- Status mapping (valid=approved, invalid=rejected)

### Setup Instructions

#### Prerequisites
- Docker and Docker Compose installed
- Go 1.21 or higher installed
- Node.js 18+ installed
- Python 3.11+ installed (for local development)
- Ports 2181, 9092, 50051, 8080, and 3000 available on localhost

#### Step 1: Start All Services
```bash
cd nexus-lite
docker-compose up -d
```

This will start:
- Zookeeper (port 2181)
- Kafka (port 9092)
- Liquidity Service (port 50051)

Wait for all services to be fully initialized (usually 30-60 seconds).

#### Step 2: Install Go Dependencies
```bash
cd nexus-lite
go mod download
go mod tidy
```

#### Step 3: Install Dashboard Dependencies
```bash
cd nexus-lite/dashboard
npm install
```

#### Step 4: Run the Producer (Bank A)
In a new terminal:
```bash
cd nexus-lite/producer
go run main.go
```

The producer will start generating and publishing transactions at 1,000 messages/second.

#### Step 5: Run the Consumer (PayNet Switch)
In another new terminal:
```bash
cd nexus-lite/consumer
go run main.go localhost:9092 localhost:50051 :8080
```

The consumer will start processing transactions with liquidity checks and broadcasting to WebSocket clients.

**Consumer Output with WebSocket:**
```
2024/01/02 14:50:05 Starting PayNet Switch Consumer (Phase 4 - Command Center Dashboard)
2024/01/02 14:50:05 Kafka Broker: localhost:9092
2024/01/02 14:50:05 Topic: nexus-transactions
2024/01/02 14:50:05 Consumer Group: paynet-switch-consumer
2024/01/02 14:50:05 Workers: 5
2024/01/02 14:50:05 Liquidity Service: localhost:50051
2024/01/02 14:50:05 WebSocket Server: :8080
2024/01/02 14:50:05 Press Ctrl+C to stop...
2024/01/02 14:50:05 [WebSocket] Server starting on :8080
```

#### Step 6: Run the Dashboard
In another new terminal:
```bash
cd nexus-lite/dashboard
npm run dev
```

The dashboard will be available at [http://localhost:3000](http://localhost:3000).

**Dashboard Features:**
- Interactive ASEAN map with bank nodes
- Real-time transaction animations
- Live statistics panel
- Recent transactions list
- Click on any transaction to view ISO 20022 XML details

### Dashboard Usage

#### Viewing Transactions
- Transactions appear in real-time on the map and list
- Click on a transaction in the list to view full XML details
- Click on bank nodes to see transaction counts

#### Understanding Status Colors
- **Green**: Approved transactions (valid XML, sufficient liquidity)
- **Red**: Rejected transactions (invalid XML or insufficient liquidity)
- **Yellow**: Pending transactions (awaiting processing)

#### XML Detail Panel
- Shows full ISO 20022 pacs.008 XML
- Syntax-highlighted for readability
- Displays error codes for rejected transactions
- Copy button to copy XML to clipboard

### Advanced Usage

#### Custom WebSocket Port
To use a different WebSocket port:
```bash
cd nexus-lite/consumer
go run main.go localhost:9092 localhost:50051 :9000
```

Then update the dashboard environment variable:
```bash
cd nexus-lite/dashboard
echo "NEXT_PUBLIC_WS_URL=ws://localhost:9000/ws" > .env.local
npm run dev
```

#### Dashboard Production Build
```bash
cd nexus-lite/dashboard
npm run build
npm start
```

### Phase 4 Technical Specifications

**Dashboard Specifications:**
- Framework: Next.js 14 with App Router
- Styling: Tailwind CSS with custom sci-fi theme
- Animations: Framer Motion
- WebSocket: Native browser WebSocket API
- Icons: Lucide React
- Fonts: JetBrains Mono (monospace), Inter (sans-serif)

**WebSocket Server Specifications:**
- Protocol: WebSocket (RFC 6455)
- Port: 8080 (default)
- Message Format: JSON
- Broadcast Rate: Real-time for transactions, 1 second for metrics
- Max Concurrent Clients: Unlimited (memory limited)

**Integration Points:**
- Consumer broadcasts each processed transaction to WebSocket
- Metrics broadcasted every second
- Dashboard subscribes to WebSocket for real-time updates
- Fail-open: Dashboard works with mock data if WebSocket unavailable

### Project Structure

```
nexus-lite/
├── dashboard/
│   ├── app/
│   │   ├── layout.tsx          # Root layout with fonts
│   │   ├── page.tsx            # Main dashboard page
│   │   └── globals.css         # Global styles and Tailwind
│   ├── components/
│   │   ├── ASEANMap.tsx       # Interactive map component
│   │   ├── StatsPanel.tsx      # Statistics display
│   │   ├── TransactionList.tsx  # Recent transactions
│   │   ├── TransactionAnimation.tsx  # Transaction animations
│   │   └── XMLDetailPanel.tsx  # XML detail slide-out
│   ├── lib/
│   │   ├── types.ts            # TypeScript type definitions
│   │   └── websocket.ts        # WebSocket client
│   ├── package.json
│   ├── tsconfig.json
│   ├── tailwind.config.ts
│   ├── next.config.js
│   ├── postcss.config.js
│   └── README.md
├── consumer/
│   ├── main.go                 # Updated with WebSocket integration
│   ├── liquidity_client.go
│   └── websocket.go           # New WebSocket server
├── producer/
│   └── main.go
├── liquidity-service/
│   └── ...
├── proto/
│   └── ...
├── docker-compose.yml
├── go.mod
└── README.md
```

### Error Codes Reference

| Error Code | Description | Scenario |
|------------|-------------|-----------|
| `AC04` | Closed Account | Bank ID not found in system |
| `AM04` | Insufficient Funds | Transaction amount exceeds available balance |
| `OK` | Approved | Sufficient funds for transaction |
| `XML_PARSE_ERROR` | XML Error | Failed to parse XML message |
| `MISSING_MSG_ID` | Validation Error | Message ID is missing |
| `INVALID_TX_COUNT` | Validation Error | Transaction count is invalid |
| `INVALID_DBTR_BIC` | Validation Error | Debtor BIC format is invalid |
| `INVALID_CDTR_BIC` | Validation Error | Creditor BIC format is invalid |
| `NO_TRANSACTIONS` | Validation Error | No transactions found in message |

### Stopping the System

To stop all services gracefully:

1. Press `Ctrl+C` in the producer terminal
2. Press `Ctrl+C` in the consumer terminal
3. Press `Ctrl+C` in the dashboard terminal
4. Stop Docker containers:
```bash
docker-compose down
```

### Troubleshooting

#### Dashboard Not Connecting to WebSocket
If the dashboard shows "DISCONNECTED":
1. Verify consumer is running with WebSocket server
2. Check WebSocket port (default 8080)
3. Check firewall settings
4. Verify WebSocket URL in `.env.local`

#### Transactions Not Appearing
If transactions are not appearing on the dashboard:
1. Verify producer is running and publishing to Kafka
2. Verify consumer is processing messages
3. Check WebSocket server logs for errors
4. Check browser console for WebSocket errors

#### High Latency
If dashboard updates are slow:
1. Check network latency between dashboard and WebSocket server
2. Reduce number of displayed transactions
3. Close other browser tabs

### Phase 4 Status
✅ Complete - Command Center Dashboard with real-time WebSocket integration ready for testing.

## License
PayNet Internal Project - Next50 George Town Accord Initiative
