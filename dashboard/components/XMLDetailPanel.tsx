"use client";

import { motion, AnimatePresence } from "framer-motion";
import { Transaction } from "@/lib/types";
import { X, Copy, Check, AlertTriangle, Clock, DollarSign } from "lucide-react";
import { useState } from "react";

interface XMLDetailPanelProps {
  transaction: Transaction | null;
  onClose: () => void;
}

export default function XMLDetailPanel({
  transaction,
  onClose,
}: XMLDetailPanelProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (transaction) {
      await navigator.clipboard.writeText(transaction.xml);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const formatXML = (xml: string) => {
    const formatted = xml
      .replace(/></g, ">\n<")
      .replace(/</g, "\n<")
      .split("\n")
      .filter((line) => line.trim())
      .map((line) => {
        const indent = (line.match(/</g) || []).length - 1;
        const spaces = "  ".repeat(Math.max(0, indent));
        return spaces + line;
      })
      .join("\n");
    return formatted;
  };

  const highlightXML = (xml: string) => {
    return xml
      .replace(/(<\/?[\w-]+)/g, '<span class="xml-tag">$1</span>')
      .replace(/(\s[\w-]+)=/g, ' <span class="xml-attribute">$1</span>=')
      .replace(/"([^"]*)"/g, '"<span class="xml-value">$1</span>"');
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "approved":
        return "text-success";
      case "rejected":
        return "text-error";
      case "pending":
        return "text-warning";
      default:
        return "text-primary";
    }
  };

  return (
    <AnimatePresence>
      {transaction && (
        <>
          {/* Backdrop */}
          <motion.div
            className="fixed inset-0 bg-black/50 backdrop-blur-sm z-40"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
          />

          {/* Panel */}
          <motion.div
            className="fixed right-0 top-0 h-full w-[600px] max-w-full bg-background border-l border-primary/20 z-50 overflow-hidden"
            initial={{ x: "100%" }}
            animate={{ x: 0 }}
            exit={{ x: "100%" }}
            transition={{ type: "spring", damping: 25, stiffness: 200 }}
          >
            {/* Header */}
            <div className="p-6 border-b border-primary/20 bg-panel-bg">
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h2 className="text-xl font-bold text-primary text-glow uppercase tracking-wider">
                    Transaction Details
                  </h2>
                  <p className="text-text-muted text-sm font-mono mt-1">
                    {transaction.msgId}
                  </p>
                </div>
                <button
                  onClick={onClose}
                  className="p-2 hover:bg-primary/10 rounded-lg transition-colors"
                >
                  <X className="w-5 h-5 text-text-muted" />
                </button>
              </div>

              {/* Status Badge */}
              <div className="flex items-center gap-2">
                <span
                  className={`px-3 py-1 rounded-full text-xs font-bold uppercase ${
                    transaction.status === "approved"
                      ? "bg-success/20 text-success"
                      : transaction.status === "rejected"
                      ? "bg-error/20 text-error"
                      : "bg-warning/20 text-warning"
                  }`}
                >
                  {transaction.status}
                </span>
                {transaction.errorCode && (
                  <span className="flex items-center gap-1 text-error text-xs font-mono">
                    <AlertTriangle className="w-3 h-3" />
                    {transaction.errorCode}
                  </span>
                )}
              </div>
            </div>

            {/* Content */}
            <div className="p-6 overflow-y-auto h-[calc(100%-200px)]">
              {/* Transaction Info */}
              <div className="grid grid-cols-2 gap-4 mb-6">
                <div className="bg-panel-bg/50 border border-primary/10 rounded-lg p-4">
                  <div className="flex items-center gap-2 text-text-muted text-xs uppercase mb-2">
                    <DollarSign className="w-3 h-3" />
                    Amount
                  </div>
                  <div className="text-2xl font-mono text-primary">
                    {transaction.amount.toLocaleString()}{" "}
                    <span className="text-sm text-text-muted">
                      {transaction.currency}
                    </span>
                  </div>
                </div>

                <div className="bg-panel-bg/50 border border-primary/10 rounded-lg p-4">
                  <div className="flex items-center gap-2 text-text-muted text-xs uppercase mb-2">
                    <Clock className="w-3 h-3" />
                    Latency
                  </div>
                  <div className="text-2xl font-mono text-primary">
                    {transaction.latency}{" "}
                    <span className="text-sm text-text-muted">ms</span>
                  </div>
                </div>
              </div>

              {/* Route Info */}
              <div className="bg-panel-bg/50 border border-primary/10 rounded-lg p-4 mb-6">
                <h3 className="text-sm font-bold text-primary uppercase mb-3">
                  Transaction Route
                </h3>
                <div className="flex items-center gap-4">
                  <div className="flex-1">
                    <div className="text-text-muted text-xs uppercase mb-1">
                      Source
                    </div>
                    <div className="font-mono text-text">{transaction.source}</div>
                    <div className="text-xs text-text-muted">
                      {transaction.sourceCountry}
                    </div>
                  </div>
                  <div className="text-primary">
                    <svg
                      className="w-6 h-6"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M13 7l5 5m0 0l-5 5m5-5H6"
                      />
                    </svg>
                  </div>
                  <div className="flex-1 text-right">
                    <div className="text-text-muted text-xs uppercase mb-1">
                      Destination
                    </div>
                    <div className="font-mono text-text">
                      {transaction.destination}
                    </div>
                    <div className="text-xs text-text-muted">
                      {transaction.destCountry}
                    </div>
                  </div>
                </div>
              </div>

              {/* Error Details */}
              {transaction.errorMsg && (
                <div className="bg-error/10 border border-error/20 rounded-lg p-4 mb-6">
                  <h3 className="text-sm font-bold text-error uppercase mb-2">
                    Error Details
                  </h3>
                  <p className="text-text text-sm font-mono">
                    {transaction.errorMsg}
                  </p>
                </div>
              )}

              {/* Timestamp */}
              <div className="text-text-muted text-xs mb-4">
                Timestamp:{" "}
                {new Date(transaction.timestamp).toLocaleString()}
              </div>

              {/* XML Content */}
              <div className="bg-panel-bg/50 border border-primary/10 rounded-lg overflow-hidden">
                <div className="flex items-center justify-between px-4 py-2 border-b border-primary/10 bg-primary/5">
                  <h3 className="text-sm font-bold text-primary uppercase">
                    ISO 20022 XML
                  </h3>
                  <button
                    onClick={handleCopy}
                    className="flex items-center gap-1 text-xs text-primary hover:text-primary/80 transition-colors"
                  >
                    {copied ? (
                      <>
                        <Check className="w-3 h-3" />
                        Copied
                      </>
                    ) : (
                      <>
                        <Copy className="w-3 h-3" />
                        Copy
                      </>
                    )}
                  </button>
                </div>
                <pre className="p-4 text-xs font-mono overflow-x-auto max-h-64">
                  <code
                    dangerouslySetInnerHTML={{
                      __html: highlightXML(formatXML(transaction.xml)),
                    }}
                  />
                </pre>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
