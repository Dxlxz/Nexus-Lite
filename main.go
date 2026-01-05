// Package main provides a simple ISO 20022 pacs.008 demonstration.
// This file shows basic XML marshaling for credit transfer messages.
// Note: This is a simplified example for illustration purposes.
package main

import (
	"encoding/xml"
	"fmt"
	"time"
)

// ISO 20022 Simplified Structure (pacs.008) - Basic demo structures
type Document struct {
	XMLName           xml.Name       `xml:"urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08 Document"`
	FIToFICstmrCdtTrf CreditTransfer `xml:"FIToFICstmrCdtTrf"`
}

type CreditTransfer struct {
	GrpHdr      GroupHeader       `xml:"GrpHdr"`
	CdtTrfTxInf []TransactionInfo `xml:"CdtTrfTxInf"`
}

type GroupHeader struct {
	MsgId   string    `xml:"MsgId"`
	CreDtTm time.Time `xml:"CreDtTm"`
	NbOfTxs int       `xml:"NbOfTxs"`
}

type TransactionInfo struct {
	PmtId          string `xml:"PmtId>InstrId"`
	IntrBkSttlmAmt Amount `xml:"IntrBkSttlmAmt"`
}

type Amount struct {
	Ccy   string `xml:"Ccy,attr"`
	Value string `xml:",chardata"`
}

// main demonstrates basic ISO 20022 XML generation.
// This is a simplified example showing the core structure.
func main() {
	// 1. Create the Transfer Data (The "Vibe" Input)
	transfer := Document{
		FIToFICstmrCdtTrf: CreditTransfer{
			GrpHdr: GroupHeader{
				MsgId:   "PAYNET-NEXUS-" + fmt.Sprint(time.Now().Unix()),
				CreDtTm: time.Now(),
				NbOfTxs: 1,
			},
			CdtTrfTxInf: []TransactionInfo{
				{
					PmtId: "TXN-001",
					IntrBkSttlmAmt: Amount{
						Ccy:   "MYR",
						Value: "1500.00", // Monthly Salary
					},
				},
			},
		},
	}

	// 2. Marshal to ISO 20022 XML (The "Lead" Output)
	output, _ := xml.MarshalIndent(transfer, "", "  ")
	fmt.Println(string(output))
}
