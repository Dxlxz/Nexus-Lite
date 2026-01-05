// Package main provides gRPC client functionality for liquidity checks.
// This module handles communication with the liquidity service to validate
// transaction amounts against bank balances.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/paynet/nexus-lite/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config Structures

// Bank represents a financial institution with basic identification details.
type Bank struct {
	ID   string `json:"id"`   // Unique bank identifier
	Name string `json:"name"` // Human-readable bank name
	BIC  string `json:"bic"`  // Bank Identifier Code (11 characters)
}

// NetworkConfig holds the complete network configuration for BIC mapping.
type NetworkConfig struct {
	Banks []Bank `json:"banks"` // List of banks in the network
}

var (
	bicMap      map[string]string // Maps BIC to bank ID for liquidity checks
	bankInfoMap map[string]Bank   // Maps bank ID to full bank information
	bicMapMutex sync.RWMutex      // Protects concurrent access to maps
)

// LoadBICMapping loads the BIC to Bank ID mapping from JSON configuration.
// This mapping is used to convert ISO 20022 BICs to internal bank identifiers
// for liquidity service communication.
func LoadBICMapping(path string) error {
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

	bicMapMutex.Lock()
	defer bicMapMutex.Unlock()

	bicMap = make(map[string]string)
	bankInfoMap = make(map[string]Bank)
	for _, bank := range config.Banks {
		bicMap[bank.BIC] = bank.ID
		bankInfoMap[bank.ID] = bank
	}

	return nil
}

// LiquidityClient wraps the gRPC client for liquidity checks.
// It provides methods to validate transactions against bank balances
// and manage account credits/debits.
type LiquidityClient struct {
	conn   *grpc.ClientConn                  // gRPC connection to liquidity service
	client proto.LiquidityCheckServiceClient // gRPC client stub
}

// NewLiquidityClient creates a new liquidity check client with connection to the service.
// It establishes a secure gRPC connection with timeout and blocking dial.
func NewLiquidityClient(address string) (*LiquidityClient, error) {
	// Set up a connection to the liquidity service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to liquidity service: %w", err)
	}

	return &LiquidityClient{
		conn:   conn,
		client: proto.NewLiquidityCheckServiceClient(conn),
	}, nil
}

// Close closes the connection to the liquidity service
func (lc *LiquidityClient) Close() error {
	return lc.conn.Close()
}

// CheckLiquidity performs a liquidity check for a transaction.
// It validates if the specified bank has sufficient balance for the transaction amount.
// Returns approval status, current balance, error code, and any communication errors.
func (lc *LiquidityClient) CheckLiquidity(ctx context.Context, bankID string, amount float64, currency string) (approved bool, balance float64, errorCode string, err error) {
	// Create request
	req := &proto.LiquidityCheckRequest{
		BankId:            bankID,
		TransactionAmount: amount,
		Currency:          currency,
	}

	// Call the service with timeout
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	resp, err := lc.client.CheckLiquidity(ctx, req)
	if err != nil {
		return false, 0, "", fmt.Errorf("liquidity check failed: %w", err)
	}

	return resp.Approved, resp.AvailableBalance, resp.ErrorCode, nil
}

// GetBalances fetches all bank balances from the liquidity service.
// Returns a slice of BankBalance protobuf messages with current account positions.
func (lc *LiquidityClient) GetBalances(ctx context.Context) ([]*proto.BankBalance, error) {
	resp, err := lc.client.GetBalances(ctx, &proto.GetBalancesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Balances, nil
}

// CreditBank credits a bank when it receives funds from a transaction.
// Updates the bank's balance and returns success status with new balance.
func (lc *LiquidityClient) CreditBank(ctx context.Context, bankID string, amount float64, currency string) (success bool, newBalance float64, err error) {
	req := &proto.CreditBankRequest{
		BankId:   bankID,
		Amount:   amount,
		Currency: currency,
	}

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	resp, err := lc.client.CreditBank(ctx, req)
	if err != nil {
		return false, 0, fmt.Errorf("credit bank failed: %w", err)
	}

	return resp.Success, resp.NewBalance, nil
}

// ExtractBankIDFromBIC extracts bank name from BIC code using loaded mapping.
// Returns the mapped bank ID or the original BIC if no mapping exists.
func ExtractBankIDFromBIC(bic string) string {
	bicMapMutex.RLock()
	defer bicMapMutex.RUnlock()

	if bankID, ok := bicMap[bic]; ok {
		return bankID
	}
	return bic // Return BIC as-is if not in map
}

// ParseAmount parses amount string to float64 for transaction processing.
func ParseAmount(amountStr string) (float64, error) {
	return strconv.ParseFloat(amountStr, 64)
}
