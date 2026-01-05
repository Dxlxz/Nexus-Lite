"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Transaction } from "@/lib/types";
import { CheckCircle, XCircle, Clock, ChevronRight, AlertTriangle } from "lucide-react";

interface TransactionListProps {
  transactions: Transaction[];
  onTransactionClick: (transaction: Transaction) => void;
  maxItems?: number;
}

export default function TransactionList({
  transactions,
  onTransactionClick,
  maxItems = 10,
}: TransactionListProps) {
  const [showErrorsOnly, setShowErrorsOnly] = useState(false);

  // Filter transactions based on error filter
  const filteredTransactions = showErrorsOnly
    ? transactions.filter((tx) => tx.status === "rejected")
    : transactions;

  const errorCount = transactions.filter((tx) => tx.status === "rejected").length;
  const getStatusIcon = (status: string) => {
    switch (status) {
      case "approved":
        return <CheckCircle className="w-4 h-4 text-success" />;
      case "rejected":
        return <XCircle className="w-4 h-4 text-error" />;
      case "pending":
        return <Clock className="w-4 h-4 text-warning" />;
      default:
        return null;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "approved":
        return "border-success/20 bg-success/5";
      case "rejected":
        return "border-error/20 bg-error/5";
      case "pending":
        return "border-warning/20 bg-warning/5";
      default:
        return "border-border bg-surface";
    }
  };

  const displayedTransactions = filteredTransactions.slice(0, maxItems);

  return (
    <div className="flex flex-col h-full bg-surface border border-border rounded overflow-hidden">
      <div className="p-3 border-b border-border flex items-center justify-between bg-background/50">
        <h3 className="text-xs font-bold text-text uppercase tracking-widest">
          Transaction Log
        </h3>
        <div className="flex items-center gap-1.5 text-[10px] text-text-muted font-mono">
          <span className="w-1.5 h-1.5 rounded-full bg-success animate-pulse" />
          REALTIME
        </div>
      </div>

      {/* Filter Bar */}
      <div className="px-3 py-2 border-b border-border/50 bg-background/30 flex items-center justify-between">
        <button
          onClick={() => setShowErrorsOnly(!showErrorsOnly)}
          className={`flex items-center gap-1.5 px-2 py-1 rounded text-[10px] font-mono uppercase transition-all ${
            showErrorsOnly
              ? "bg-error/20 text-error border border-error/30"
              : "bg-surface text-text-muted border border-border hover:border-error/30 hover:text-error"
          }`}
        >
          <AlertTriangle className="w-3 h-3" />
          <span>Errors Only</span>
          {errorCount > 0 && (
            <span className={`ml-1 px-1.5 py-0.5 rounded-full text-[9px] ${
              showErrorsOnly ? "bg-error/30" : "bg-error/20 text-error"
            }`}>
              {errorCount}
            </span>
          )}
        </button>
        <span className="text-[9px] text-text-muted font-mono">
          {showErrorsOnly ? `${filteredTransactions.length} errors` : "All"}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto">
        <AnimatePresence mode="popLayout">
          {displayedTransactions.map((tx) => (
            <motion.div
              key={tx.id}
              className={`border-b border-border/50 px-3 py-2 cursor-pointer transition-colors hover:bg-primary/5 ${getStatusColor(
                tx.status
              )}`}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => onTransactionClick(tx)}
            >
              <div className="flex items-center justify-between gap-4">
                <div className="flex items-center gap-2 min-w-0">
                  {getStatusIcon(tx.status)}
                  <span className="text-[10px] font-mono text-text-muted truncate">
                    {tx.msgId.slice(-8)}
                  </span>
                </div>
                <div className="text-[10px] text-text-muted font-mono">
                  {new Date(tx.timestamp).toLocaleTimeString([], { hour12: false })}
                </div>
              </div>
              
              <div className="flex items-center justify-between mt-1">
                <div className="flex items-center gap-1 text-xs truncate min-w-0">
                  <span className="text-text font-bold">{tx.sourceCountry}</span>
                  <ChevronRight className="w-2.5 h-2.5 text-text-muted" />
                  <span className="text-text font-bold">{tx.destCountry}</span>
                </div>
                <div className="text-xs font-mono font-bold text-primary">
                  {tx.amount.toLocaleString(undefined, { minimumFractionDigits: 0 })}
                </div>
              </div>
            </motion.div>
          ))}
        </AnimatePresence>

        {displayedTransactions.length === 0 && (
          <div className="text-center py-12 text-text-muted">
            <p className="text-xs font-mono uppercase tracking-tighter">Initializing stream...</p>
          </div>
        )}
      </div>

      <div className="p-2 border-t border-border bg-background/50 text-[10px] text-text-muted font-mono text-center">
        {showErrorsOnly ? (
          <>ERRORS: {filteredTransactions.length} | TOTAL: {transactions.length}</>
        ) : (
          <>TOTAL: {transactions.length} | LIMIT: {maxItems}</>
        )}
      </div>
    </div>
  );
}
