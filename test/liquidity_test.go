package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"testing"
	"time"

	"github.com/paynet/nexus-lite/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// TestLiquidityCheckService tests the liquidity check gRPC service
func TestLiquidityCheckService(t *testing.T) {
	// This test assumes the liquidity service is running on localhost:50051
	// In a real test environment, you would start a test server
	
	conn, err := grpc.Dial("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer conn.Close()

	client := proto.NewLiquidityCheckServiceClient(conn)

	tests := []struct {
		name           string
		bankID         string
		amount         float64
		currency       string
		wantApproved   bool
		wantErrorCode  string
	}{
		{
			name:           "Normal transaction - Approved",
			bankID:         "MAYBANK",
			amount:         1000.00,
			currency:       "MYR",
			wantApproved:   true,
			wantErrorCode:  "OK",
		},
		{
			name:           "Insufficient funds - Rejected",
			bankID:         "AMBANK",
			amount:         5000000.00, // More than AMBANK's balance of 1.9M
			currency:       "MYR",
			wantApproved:   false,
			wantErrorCode:  "AM04",
		},
		{
			name:           "Unknown bank - Rejected",
			bankID:         "UNKNOWN_BANK",
			amount:         100.00,
			currency:       "MYR",
			wantApproved:   false,
			wantErrorCode:  "AC04",
		},
		{
			name:           "Large but approved transaction",
			bankID:         "HSBC",
			amount:         5000000.00,
			currency:       "MYR",
			wantApproved:   true,
			wantErrorCode:  "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			req := &proto.LiquidityCheckRequest{
				BankId:           tt.bankID,
				TransactionAmount: tt.amount,
				Currency:         tt.currency,
			}

			resp, err := client.CheckLiquidity(ctx, req)
			if err != nil {
				t.Fatalf("CheckLiquidity() error = %v", err)
			}

			if resp.Approved != tt.wantApproved {
				t.Errorf("CheckLiquidity() approved = %v, want %v", resp.Approved, tt.wantApproved)
			}

			if resp.ErrorCode != tt.wantErrorCode {
				t.Errorf("CheckLiquidity() errorCode = %v, want %v", resp.ErrorCode, tt.wantErrorCode)
			}

			t.Logf("Test '%s': Approved=%v, ErrorCode=%s, Balance=%.2f",
				tt.name, resp.Approved, resp.ErrorCode, resp.AvailableBalance)
		})
	}
}

// TestLiquidityClient tests the Go liquidity client wrapper
func TestLiquidityClient(t *testing.T) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name           string
		bankID         string
		amount         float64
		currency       string
		wantApproved   bool
		wantErrorCode  string
	}{
		{
			name:           "Normal transaction",
			bankID:         "CIMB",
			amount:         500.00,
			currency:       "MYR",
			wantApproved:   true,
			wantErrorCode:  "OK",
		},
		{
			name:           "Insufficient funds",
			bankID:         "RHB",
			amount:         10000000.00,
			currency:       "MYR",
			wantApproved:   false,
			wantErrorCode:  "AM04",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			approved, _, errorCode, err := client.CheckLiquidity(ctx, tt.bankID, tt.amount, tt.currency)
			if err != nil {
				t.Fatalf("CheckLiquidity() error = %v", err)
			}

			if approved != tt.wantApproved {
				t.Errorf("CheckLiquidity() approved = %v, want %v", approved, tt.wantApproved)
			}

			if errorCode != tt.wantErrorCode {
				t.Errorf("CheckLiquidity() errorCode = %v, want %v", errorCode, tt.wantErrorCode)
			}
		})
	}
}

// TestExtractBankIDFromBIC tests the BIC to bank ID mapping
func TestExtractBankIDFromBIC(t *testing.T) {
	tests := []struct {
		name  string
		bic   string
		want  string
	}{
		{
			name: "Maybank BIC",
			bic:  "MBBEMYKL",
			want: "MAYBANK",
		},
		{
			name: "CIMB BIC",
			bic:  "CIBBMYKL",
			want: "CIMB",
		},
		{
			name: "Public Bank BIC",
			bic:  "PBBEMYKL",
			want: "PUBLIC_BANK",
		},
		{
			name: "Unknown BIC",
			bic:  "UNKNOWNXX",
			want: "UNKNOWNXX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBankIDFromBIC(tt.bic)
			if got != tt.want {
				t.Errorf("ExtractBankIDFromBIC() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckLiquidityIntegration tests the full integration flow
func TestCheckLiquidityIntegration(t *testing.T) {
	// Create a sample ISO 20022 transaction
	doc := Document{
		FIToFICstmrCdtTrf: CreditTransfer{
			GrpHdr: GroupHeader{
				MsgId:    "TEST-MSG-001",
				CreDtTm:  time.Now(),
				NbOfTxs:  1,
				InitgPty: "Test Bank",
				Dbtr:     "Test Debtor",
				DbtrAcct: "ACC123456",
				DbtrAgt:  "MBBEMYKL", // Maybank BIC
				CdtrAgt:  "DBSSSGSG",
			},
			CdtTrfTxInf: []TransactionInfo{
				{
					PmtId:      "TXN-001",
					EndToEndId: "E2E-001",
					IntrBkSttlmAmt: Amount{
						Ccy:   "MYR",
						Value: "1000.00",
					},
					Cdtr:     "Test Creditor",
					CdtrAcct: "ACC789012",
				},
			},
		},
	}

	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal document: %v", err)
	}

	// Test liquidity check
	approved, errorCode, errorMsg, balance := checkLiquidity(data)

	t.Logf("Liquidity check result: Approved=%v, ErrorCode=%s, ErrorMsg=%s, Balance=%.2f",
		approved, errorCode, errorMsg, balance)

	// Maybank should have sufficient funds for 1000 MYR
	if !approved {
		t.Errorf("Expected transaction to be approved, got rejected with code: %s", errorCode)
	}

	if errorCode != "" {
		t.Errorf("Expected no error code, got: %s", errorCode)
	}
}

// TestSequentialTransactions tests multiple transactions draining balance
func TestSequentialTransactions(t *testing.T) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bankID := "PUBLIC_BANK"
	initialBalance := 4200000.00 // From model.py
	transactionAmount := 100000.00

	// Execute multiple transactions
	for i := 0; i < 10; i++ {
		approved, balance, errorCode, err := client.CheckLiquidity(ctx, bankID, transactionAmount, "MYR")
		if err != nil {
			t.Fatalf("Transaction %d failed: %v", i, err)
		}

		expectedBalance := initialBalance - float64(i+1)*transactionAmount

		if !approved {
			t.Errorf("Transaction %d: Expected approval, got rejected with code: %s", i, errorCode)
		}

		if balance != expectedBalance {
			t.Errorf("Transaction %d: Expected balance %.2f, got %.2f", i, expectedBalance, balance)
		}

		t.Logf("Transaction %d: Approved=%v, Balance=%.2f", i, approved, balance)
	}
}

// TestLiquidityCheckTimeout tests timeout handling
func TestLiquidityCheckTimeout(t *testing.T) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	// Very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, _, _, err = client.CheckLiquidity(ctx, "MAYBANK", 100.00, "MYR")
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	t.Logf("Timeout test: Got expected error: %v", err)
}

// BenchmarkLiquidityCheck benchmarks the liquidity check performance
func BenchmarkLiquidityCheck(b *testing.B) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		b.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = client.CheckLiquidity(ctx, "MAYBANK", 1000.00, "MYR")
	}
}

// TestInvalidAmount tests handling of invalid amounts
func TestInvalidAmount(t *testing.T) {
	doc := Document{
		FIToFICstmrCdtTrf: CreditTransfer{
			GrpHdr: GroupHeader{
				MsgId:    "TEST-MSG-002",
				CreDtTm:  time.Now(),
				NbOfTxs:  1,
				DbtrAgt:  "MBBEMYKL",
			},
			CdtTrfTxInf: []TransactionInfo{
				{
					IntrBkSttlmAmt: Amount{
						Ccy:   "MYR",
						Value: "invalid", // Invalid amount
					},
				},
			},
		},
	}

	data, _ := xml.MarshalIndent(doc, "", "  ")
	approved, errorCode, errorMsg, _ := checkLiquidity(data)

	if approved {
		t.Error("Expected transaction to be rejected due to invalid amount")
	}

	if errorCode != "LIQUIDITY_CHECK_ERROR" {
		t.Errorf("Expected LIQUIDITY_CHECK_ERROR, got: %s", errorCode)
	}

	t.Logf("Invalid amount test: ErrorCode=%s, ErrorMsg=%s", errorCode, errorMsg)
}

// TestEmptyTransaction tests handling of empty transactions
func TestEmptyTransaction(t *testing.T) {
	doc := Document{
		FIToFICstmrCdtTrf: CreditTransfer{
			GrpHdr: GroupHeader{
				MsgId:   "TEST-MSG-003",
				CreDtTm: time.Now(),
				NbOfTxs: 0, // No transactions
				DbtrAgt: "MBBEMYKL",
			},
			CdtTrfTxInf: []TransactionInfo{}, // Empty
		},
	}

	data, _ := xml.MarshalIndent(doc, "", "  ")
	approved, errorCode, errorMsg, _ := checkLiquidity(data)

	// Empty transactions should be approved (nothing to check)
	if !approved {
		t.Errorf("Expected empty transaction to be approved, got: %s", errorMsg)
	}

	t.Logf("Empty transaction test: Approved=%v, ErrorCode=%s", approved, errorCode)
}

// TestCurrencyVariation tests different currency codes
func TestCurrencyVariation(t *testing.T) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	currencies := []string{"MYR", "USD", "SGD", "EUR"}

	for _, currency := range currencies {
		t.Run(fmt.Sprintf("Currency_%s", currency), func(t *testing.T) {
			approved, _, _, err := client.CheckLiquidity(ctx, "MAYBANK", 100.00, currency)
			if err != nil {
				t.Fatalf("CheckLiquidity() error = %v", err)
			}

			if !approved {
				t.Errorf("Expected approval for currency %s", currency)
			}

			t.Logf("Currency %s: Approved=%v", currency, approved)
		})
	}
}

// TestMultipleBanks tests liquidity check across multiple banks
func TestMultipleBanks(t *testing.T) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	banks := []string{"MAYBANK", "CIMB", "RHB", "PUBLIC_BANK", "AMBANK", "HSBC"}

	for _, bank := range banks {
		t.Run(fmt.Sprintf("Bank_%s", bank), func(t *testing.T) {
			approved, balance, _, err := client.CheckLiquidity(ctx, bank, 1000.00, "MYR")
			if err != nil {
				t.Fatalf("CheckLiquidity() error = %v", err)
			}

			if !approved {
				t.Errorf("Expected approval for bank %s", bank)
			}

			if balance < 0 {
				t.Errorf("Expected non-negative balance for bank %s, got: %.2f", bank, balance)
			}

			t.Logf("Bank %s: Approved=%v, Balance=%.2f", bank, approved, balance)
		})
	}
}

// TestLargeTransaction tests handling of very large transactions
func TestLargeTransaction(t *testing.T) {
	client, err := NewLiquidityClient("localhost:50051")
	if err != nil {
		t.Skipf("Liquidity service not available: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Very large transaction that should exceed any bank's balance
	approved, balance, errorCode, err := client.CheckLiquidity(ctx, "MAYBANK", 100000000.00, "MYR")
	if err != nil {
		t.Fatalf("CheckLiquidity() error = %v", err)
	}

	if approved {
		t.Error("Expected rejection for very large transaction")
	}

	if errorCode != "AM04" {
		t.Errorf("Expected AM04 error code, got: %s", errorCode)
	}

	t.Logf("Large transaction: Approved=%v, Balance=%.2f, ErrorCode=%s", approved, balance, errorCode)
}
