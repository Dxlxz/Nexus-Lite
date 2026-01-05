"use client";

import { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import StatsPanel from "@/components/StatsPanel";
import TransactionList from "@/components/TransactionList";
import XMLDetailPanel from "@/components/XMLDetailPanel";
import { getWebSocketClient } from "@/lib/websocket";
import { Transaction, BankNode, BankBalance } from "@/lib/types";
import { Activity, Zap, Globe, Database } from "lucide-react";

// Dynamically import ASEANMap to avoid SSR issues with Leaflet
const ASEANMap = dynamic(() => import("@/components/ASEANMap"), {
  ssr: false,
  loading: () => (
    <div className="w-full h-full flex items-center justify-center bg-surface border border-border rounded">
      <div className="flex flex-col items-center gap-2">
        <Activity className="w-6 h-6 text-primary animate-pulse" />
        <span className="text-xs font-mono text-text-muted uppercase">Initializing Geospatial Layer...</span>
      </div>
    </div>
  ),
});

// Mock data for initial state
const mockBankNodes: BankNode[] = [
  { id: "MY-MBB", countryId: "MY", name: "Maybank", bic: "MBBEMYKL", lat: 3.1390, lng: 101.6869, transactionCount: 0 },
  { id: "SG-DBS", countryId: "SG", name: "DBS Bank", bic: "DBSSSGSG", lat: 1.3521, lng: 103.8198, transactionCount: 0 },
  { id: "TH-BBL", countryId: "TH", name: "Bangkok Bank", bic: "BKKBTHBK", lat: 13.7563, lng: 100.5018, transactionCount: 0 },
  { id: "ID-MRI", countryId: "ID", name: "Bank Mandiri", bic: "BMRIIDJA", lat: -6.2088, lng: 106.8456, transactionCount: 0 },
  { id: "PH-BPI", countryId: "PH", name: "BPI", bic: "BPIPHMM", lat: 14.5995, lng: 120.9842, transactionCount: 0 },
  { id: "VN-VCB", countryId: "VN", name: "Vietcombank", bic: "BKVNVNVX", lat: 21.0285, lng: 105.8542, transactionCount: 0 },
];

const mockBankBalances: BankBalance[] = [
  // Malaysia
  { bankName: "Maybank", bic: "MBBEMYKL", balance: 2500000000, currency: "MYR", country: "MY", countryName: "Malaysia" },
  { bankName: "CIMB", bic: "CIBBMYKL", balance: 1800000000, currency: "MYR", country: "MY", countryName: "Malaysia" },
  { bankName: "Public Bank", bic: "PBBEMYKL", balance: 1600000000, currency: "MYR", country: "MY", countryName: "Malaysia" },
  { bankName: "RHB Bank", bic: "RHBBMYKL", balance: 1200000000, currency: "MYR", country: "MY", countryName: "Malaysia" },
  { bankName: "Hong Leong Bank", bic: "HLBBMYKL", balance: 950000000, currency: "MYR", country: "MY", countryName: "Malaysia" },
  // Singapore
  { bankName: "DBS Bank", bic: "DBSSSGSG", balance: 8500000000, currency: "SGD", country: "SG", countryName: "Singapore" },
  { bankName: "UOB", bic: "UOVBSGSG", balance: 6200000000, currency: "SGD", country: "SG", countryName: "Singapore" },
  { bankName: "OCBC", bic: "OCBCSGSG", balance: 4200000000, currency: "SGD", country: "SG", countryName: "Singapore" },
  // Thailand
  { bankName: "Bangkok Bank", bic: "BKKBTHBK", balance: 180000000000, currency: "THB", country: "TH", countryName: "Thailand" },
  { bankName: "Kasikornbank", bic: "KASITHBK", balance: 140000000000, currency: "THB", country: "TH", countryName: "Thailand" },
  { bankName: "Krungthai Bank", bic: "KRTHTHBK", balance: 120000000000, currency: "THB", country: "TH", countryName: "Thailand" },
  // Indonesia
  { bankName: "Bank Mandiri", bic: "BMRIIDJA", balance: 380000000000000, currency: "IDR", country: "ID", countryName: "Indonesia" },
  { bankName: "BCA", bic: "CENAIDJA", balance: 280000000000000, currency: "IDR", country: "ID", countryName: "Indonesia" },
  { bankName: "BNI", bic: "BNINIDJA", balance: 240000000000000, currency: "IDR", country: "ID", countryName: "Indonesia" },
  // Philippines
  { bankName: "BPI", bic: "BPIPHMM", balance: 95000000000, currency: "PHP", country: "PH", countryName: "Philippines" },
  { bankName: "Metrobank", bic: "MBTCPHMM", balance: 78000000000, currency: "PHP", country: "PH", countryName: "Philippines" },
  { bankName: "BDO", bic: "BNORPHMM", balance: 82000000000, currency: "PHP", country: "PH", countryName: "Philippines" },
  // Vietnam
  { bankName: "Vietcombank", bic: "BFTVVNVX", balance: 520000000000000, currency: "VND", country: "VN", countryName: "Vietnam" },
  { bankName: "Vietinbank", bic: "ICBVVNVX", balance: 420000000000000, currency: "VND", country: "VN", countryName: "Vietnam" },
  { bankName: "Techcombank", bic: "VTCBVNVX", balance: 380000000000000, currency: "VND", country: "VN", countryName: "Vietnam" },
];

export default function Dashboard() {
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [selectedNode, setSelectedNode] = useState<BankNode | null>(null);
  const [selectedTransaction, setSelectedTransaction] = useState<Transaction | null>(null);
  const [bankNodes, setBankNodes] = useState<BankNode[]>(mockBankNodes);
  const [bankBalances, setBankBalances] = useState<BankBalance[]>(mockBankBalances);
  const [lastLiquidityUpdate, setLastLiquidityUpdate] = useState<Date>(new Date());
  const [isConnected, setIsConnected] = useState(false);

  // Metrics state
  const [totalProcessed, setTotalProcessed] = useState(0);
  const [messagesPerSecond, setMessagesPerSecond] = useState(0);
  const [tpsHistory, setTpsHistory] = useState<number[]>(new Array(20).fill(0));
  const [successRate, setSuccessRate] = useState(100);
  const [activeConnections, setActiveConnections] = useState(0);
  const [currentTime, setCurrentTime] = useState<string>("");

  useEffect(() => {
    // Initial time set
    setCurrentTime(new Date().toLocaleTimeString([], { hour12: false }));

    const timer = setInterval(() => {
      setCurrentTime(new Date().toLocaleTimeString([], { hour12: false }));
    }, 1000);

    const wsClient = getWebSocketClient();

    // Subscribe to connection status
    const unsubscribeConnection = wsClient.onConnection((connected) => {
      setIsConnected(connected);
    });

    // Subscribe to messages
    const unsubscribeMessage = wsClient.onMessage((message) => {
      if (message.type === "transaction") {
        const tx = message.data as Transaction;
        setTransactions((prev) => [tx, ...prev].slice(0, 100));
        setTotalProcessed((prev) => prev + 1);

        // Update bank node transaction counts
        setBankNodes((prev) =>
          prev.map((node) => {
            if (node.countryId === tx.sourceCountry || node.countryId === tx.destCountry) {
              return { ...node, transactionCount: node.transactionCount + 1 };
            }
            return node;
          })
        );
      } else if (message.type === "metrics") {
        const metrics = message.data as any;
        const newTps = metrics.messagesPerSecond || 0;
        setMessagesPerSecond(newTps);
        setTpsHistory(prev => [...prev.slice(1), newTps]);
        setSuccessRate(metrics.successRate || 100);
        setActiveConnections(metrics.activeConnections || 0);
      } else if (message.type === "balances") {
        setBankBalances(message.data as BankBalance[]);
        setLastLiquidityUpdate(new Date());
      }
    });

    // Simulate initial data for demo
    const simulateInitialData = () => {
      const demoTransactions: Transaction[] = [
        {
          id: "tx-1",
          msgId: "PAYNET-NEXUS-1704201800-0001",
          source: "Maybank",
          sourceCountry: "MY",
          destination: "DBS Bank",
          destCountry: "SG",
          amount: 15000,
          currency: "MYR",
          status: "approved",
          timestamp: new Date(),
          xml: `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08">
  <FIToFICstmrCdtTrf>
    <GrpHdr>
      <MsgId>PAYNET-NEXUS-1704201800-0001</MsgId>
      <CreDtTm>2024-01-02T14:50:00Z</CreDtTm>
      <NbOfTxs>1</NbOfTxs>
      <InitgPty><Nm>Maybank</Nm></InitgPty>
      <Dbtr><Nm>Maybank</Nm></Dbtr>
      <DbtrAgt><FinInstnId><BICFI>MBBEMYKL</BICFI></FinInstnId></DbtrAgt>
      <CdtrAgt><FinInstnId><BICFI>DBSSSGSG</BICFI></FinInstnId></CdtrAgt>
    </GrpHdr>
    <CdtTrfTxInf>
      <PmtId>
        <InstrId>TXN-001</InstrId>
        <EndToEndId>E2E-001</EndToEndId>
      </PmtId>
      <IntrBkSttlmAmt Ccy="MYR">15000.00</IntrBkSttlmAmt>
      <Cdtr><Nm>DBS Bank</Nm></Cdtr>
      <CdtrAcct><Id><Othr><Id>SG123456789</Id></Othr></Id></CdtrAcct>
    </CdtTrfTxInf>
  </FIToFICstmrCdtTrf>
</Document>`,
          latency: 45,
        },
        {
          id: "tx-2",
          msgId: "PAYNET-NEXUS-1704201800-0002",
          source: "Bangkok Bank",
          sourceCountry: "TH",
          destination: "Bank Mandiri",
          destCountry: "ID",
          amount: 25000,
          currency: "THB",
          status: "approved",
          timestamp: new Date(Date.now() - 5000),
          xml: `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08">
  <FIToFICstmrCdtTrf>
    <GrpHdr>
      <MsgId>PAYNET-NEXUS-1704201800-0002</MsgId>
      <CreDtTm>2024-01-02T14:49:55Z</CreDtTm>
      <NbOfTxs>1</NbOfTxs>
      <InitgPty><Nm>Bangkok Bank</Nm></InitgPty>
      <Dbtr><Nm>Bangkok Bank</Nm></Dbtr>
      <DbtrAgt><FinInstnId><BICFI>BKKBTHBK</BICFI></FinInstnId></DbtrAgt>
      <CdtrAgt><FinInstnId><BICFI>BMRIIDJA</BICFI></FinInstnId></CdtrAgt>
    </GrpHdr>
    <CdtTrfTxInf>
      <PmtId>
        <InstrId>TXN-002</InstrId>
        <EndToEndId>E2E-002</EndToEndId>
      </PmtId>
      <IntrBkSttlmAmt Ccy="THB">25000.00</IntrBkSttlmAmt>
      <Cdtr><Nm>Bank Mandiri</Nm></Cdtr>
      <CdtrAcct><Id><Othr><Id>ID123456789</Id></Othr></Id></CdtrAcct>
    </CdtTrfTxInf>
  </FIToFICstmrCdtTrf>
</Document>`,
          latency: 52,
        },
        {
          id: "tx-3",
          msgId: "PAYNET-NEXUS-1704201800-0003",
          source: "BPI",
          sourceCountry: "PH",
          destination: "Vietcombank",
          destCountry: "VN",
          amount: 8000,
          currency: "PHP",
          status: "rejected",
          errorCode: "AM04",
          errorMsg: "Liquidity check rejected: AM04",
          timestamp: new Date(Date.now() - 10000),
          xml: `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08">
  <FIToFICstmrCdtTrf>
    <GrpHdr>
      <MsgId>PAYNET-NEXUS-1704201800-0003</MsgId>
      <CreDtTm>2024-01-02T14:49:50Z</CreDtTm>
      <NbOfTxs>1</NbOfTxs>
      <InitgPty><Nm>BPI</Nm></InitgPty>
      <Dbtr><Nm>BPI</Nm></Dbtr>
      <DbtrAgt><FinInstnId><BICFI>BPIPHMM</BICFI></FinInstnId></DbtrAgt>
      <CdtrAgt><FinInstnId><BICFI>BKVNVNVX</BICFI></FinInstnId></CdtrAgt>
    </GrpHdr>
    <CdtTrfTxInf>
      <PmtId>
        <InstrId>TXN-003</InstrId>
        <EndToEndId>E2E-003</EndToEndId>
      </PmtId>
      <IntrBkSttlmAmt Ccy="PHP">8000.00</IntrBkSttlmAmt>
      <Cdtr><Nm>Vietcombank</Nm></Cdtr>
      <CdtrAcct><Id><Othr><Id>VN123456789</Id></Othr></Id></CdtrAcct>
    </CdtTrfTxInf>
  </FIToFICstmrCdtTrf>
</Document>`,
          latency: 38,
        },
      ];

      setTransactions(demoTransactions);
      setTotalProcessed(3);
      setMessagesPerSecond(0.5);
      setSuccessRate(66.7);
      setActiveConnections(6);
    };

    simulateInitialData();

    return () => {
      clearInterval(timer);
      unsubscribeConnection();
      unsubscribeMessage();
    };
  }, []);

  const handleNodeClick = (node: BankNode) => {
    setSelectedNode(node);
  };

  const handleTransactionClick = (tx: Transaction) => {
    setSelectedTransaction(tx);
  };

  const handleRefreshLiquidity = () => {
    // Request fresh liquidity data from websocket or API
    const wsClient = getWebSocketClient();
    wsClient.send({ type: "request_balances" });
    // Update timestamp immediately for UX
    setLastLiquidityUpdate(new Date());
  };

  return (
    <div className="h-screen w-screen bg-background text-text font-sans overflow-hidden flex flex-col selection:bg-primary/30">
      {/* Subtle Grid Background */}
      <div className="fixed inset-0 data-grid-bg pointer-events-none" />

      {/* Professional Header - Fixed Height */}
      <header className="h-12 border-b border-border bg-surface flex items-center justify-between px-4 shrink-0 z-20">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="bg-primary/10 p-1.5 rounded-sm border border-primary/20">
              <Activity className="w-4 h-4 text-primary" />
            </div>
            <h1 className="text-sm font-bold text-text uppercase tracking-wider">
              Nexus<span className="text-text-muted mx-1">/</span>Lite
            </h1>
          </div>
          <div className="h-4 w-px bg-border" />
          <div className="flex items-center gap-2 text-[10px] font-mono uppercase text-text-muted">
            <span className={isConnected ? "text-success" : "text-error"}>● {isConnected ? "SYS_ONLINE" : "SYS_OFFLINE"}</span>
            <span className="hidden xl:inline">| SETTLEMENT_ENGINE_V4</span>
          </div>
        </div>
        
        <div className="flex items-center gap-3">
          <div className="bg-background border border-border rounded px-2 py-1 text-[10px] font-mono text-text-muted flex items-center gap-2">
            <span>WS: {activeConnections}</span>
            <span className="w-px h-2 bg-border" />
            <span>TPS: {messagesPerSecond.toFixed(1)}</span>
          </div>
          <div className="text-[10px] font-mono text-text-muted w-[60px] text-right">
            {currentTime} UTC
          </div>
        </div>
      </header>

      {/* Main Dashboard Content - Flex Grow with Grid */}
      <main className="flex-1 p-2 overflow-hidden">
        <div className="grid grid-cols-12 gap-2 h-full">
          {/* Left Panel: Metrics & Balances (2 cols) */}
          <div className="col-span-12 lg:col-span-3 xl:col-span-2 flex flex-col gap-2 h-full overflow-hidden">
             <StatsPanel
              totalProcessed={totalProcessed}
              messagesPerSecond={messagesPerSecond}
              tpsHistory={tpsHistory}
              successRate={successRate}
              activeConnections={activeConnections}
              bankBalances={bankBalances}
              onRefreshLiquidity={handleRefreshLiquidity}
              lastLiquidityUpdate={lastLiquidityUpdate}
            />
          </div>

          {/* Center Panel: Network Map (6 cols) */}
          <div className="col-span-12 lg:col-span-5 xl:col-span-6 h-full flex flex-col gap-2 min-h-0">
            <div className="flex-1 bg-surface border border-border rounded overflow-hidden relative min-h-0">
               <ASEANMap
                countries={[]}
                bankNodes={bankNodes}
                transactions={transactions}
                onNodeClick={handleNodeClick}
                selectedNode={selectedNode}
              />
            </div>
            {/* Context Panel - Fixed Height at Bottom of Center */}
            <div className="h-32 bg-surface border border-border rounded p-3 shrink-0 overflow-y-auto">
                {selectedNode ? (
                  <div className="h-full flex flex-col">
                    <div className="flex items-center justify-between mb-2 border-b border-border/50 pb-1">
                      <span className="text-xs font-bold text-primary">{selectedNode.name}</span>
                      <span className="text-[10px] font-mono text-text-muted">{selectedNode.bic}</span>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                       <div>
                         <div className="text-[10px] text-text-muted uppercase">Volume (1h)</div>
                         <div className="text-lg font-mono text-text">{selectedNode.transactionCount}</div>
                       </div>
                       <div>
                         <div className="text-[10px] text-text-muted uppercase">Status</div>
                         <div className="text-xs text-success">Active</div>
                       </div>
                    </div>
                  </div>
                ) : (
                  <div className="h-full flex items-center justify-center text-[10px] text-text-muted uppercase tracking-widest">
                    Select a node to view telemetry
                  </div>
                )}
            </div>
          </div>

          {/* Right Panel: Transaction Log (4 cols) */}
          <div className="col-span-12 lg:col-span-4 xl:col-span-4 h-full overflow-hidden">
            <TransactionList
              transactions={transactions}
              onTransactionClick={handleTransactionClick}
              maxItems={50}
            />
          </div>
        </div>
      </main>

      {/* Detail Overlay */}
      <XMLDetailPanel
        transaction={selectedTransaction}
        onClose={() => setSelectedTransaction(null)}
      />
    </div>
  );
}
