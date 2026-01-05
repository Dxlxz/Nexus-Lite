"use client";

import { useState, useEffect } from "react";
import { motion } from "framer-motion";
import { Activity, TrendingUp, Zap, Server, DollarSign, RefreshCw } from "lucide-react";
import { BankBalance } from "@/lib/types";

interface StatsPanelProps {
  totalProcessed: number;
  messagesPerSecond: number;
  tpsHistory?: number[];
  successRate: number;
  activeConnections: number;
  bankBalances: BankBalance[];
  onRefreshLiquidity?: () => void;
  lastLiquidityUpdate?: Date;
}

// Simple Sparkline Component
const Sparkline = ({ data, color }: { data: number[], color: string }) => {
  if (!data || data.length < 2) return null;
  
  const width = 60;
  const height = 20;
  const max = Math.max(...data, 10); // Ensure minimal scale
  const min = Math.min(...data);
  const range = max - min || 1;
  
  const points = data.map((d, i) => {
    const x = (i / (data.length - 1)) * width;
    const y = height - ((d - min) / range) * height;
    return `${x},${y}`;
  }).join(" ");

  return (
    <svg width={width} height={height} className="overflow-visible">
      <polyline
        points={points}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        className={color}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
};

// Liquidity Progress Bar Component
const LiquidityBar = ({ balance, maxBalance }: { balance: number; maxBalance: number }) => {
  const percentage = Math.min((balance / maxBalance) * 100, 100);
  const getColor = () => {
    if (percentage >= 70) return "bg-success";
    if (percentage >= 40) return "bg-warning";
    return "bg-error";
  };

  return (
    <div className="w-full h-1.5 bg-border/50 rounded-full overflow-hidden mt-1">
      <motion.div
        className={`h-full ${getColor()} rounded-full`}
        initial={{ width: 0 }}
        animate={{ width: `${percentage}%` }}
        transition={{ duration: 0.5, ease: "easeOut" }}
      />
    </div>
  );
};

// Countdown Timer Component
const CountdownTimer = ({
  lastUpdate,
  intervalMinutes = 5,
  onRefresh
}: {
  lastUpdate?: Date;
  intervalMinutes?: number;
  onRefresh?: () => void;
}) => {
  const [timeLeft, setTimeLeft] = useState(intervalMinutes * 60);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    if (lastUpdate) {
      const elapsed = Math.floor((Date.now() - lastUpdate.getTime()) / 1000);
      const remaining = Math.max(0, (intervalMinutes * 60) - elapsed);
      setTimeLeft(remaining);
    }
  }, [lastUpdate, intervalMinutes]);

  useEffect(() => {
    const timer = setInterval(() => {
      setTimeLeft((prev) => {
        if (prev <= 1) {
          // Auto-refresh when timer hits 0
          if (onRefresh) {
            setIsRefreshing(true);
            onRefresh();
            setTimeout(() => setIsRefreshing(false), 1000);
          }
          return intervalMinutes * 60;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [intervalMinutes, onRefresh]);

  const minutes = Math.floor(timeLeft / 60);
  const seconds = timeLeft % 60;
  const progress = ((intervalMinutes * 60 - timeLeft) / (intervalMinutes * 60)) * 100;

  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1 bg-border/30 rounded-full overflow-hidden">
        <motion.div
          className="h-full bg-primary/50 rounded-full"
          style={{ width: `${progress}%` }}
        />
      </div>
      <div className="flex items-center gap-1">
        <span className="text-[9px] font-mono text-text-muted">
          {String(minutes).padStart(2, '0')}:{String(seconds).padStart(2, '0')}
        </span>
        <RefreshCw
          className={`w-3 h-3 text-text-muted cursor-pointer hover:text-primary transition-colors ${isRefreshing ? 'animate-spin' : ''}`}
          onClick={() => {
            if (onRefresh) {
              setIsRefreshing(true);
              onRefresh();
              setTimeLeft(intervalMinutes * 60);
              setTimeout(() => setIsRefreshing(false), 1000);
            }
          }}
        />
      </div>
    </div>
  );
};

// Country flag mapping
const countryFlags: Record<string, string> = {
  MY: "🇲🇾",
  SG: "🇸🇬",
  TH: "🇹🇭",
  ID: "🇮🇩",
  PH: "🇵🇭",
  VN: "🇻🇳",
};

// Group banks by country
const groupBanksByCountry = (banks: BankBalance[]) => {
  const grouped: Record<string, BankBalance[]> = {};
  banks.forEach((bank) => {
    const country = bank.country || "OTHER";
    if (!grouped[country]) {
      grouped[country] = [];
    }
    grouped[country].push(bank);
  });
  return grouped;
};

// Format large numbers compactly
const formatBalance = (balance: number, currency: string) => {
  if (currency === "IDR" || currency === "VND") {
    // Show in billions for IDR/VND
    if (balance >= 1e12) {
      return `${(balance / 1e12).toFixed(1)}T`;
    }
    if (balance >= 1e9) {
      return `${(balance / 1e9).toFixed(1)}B`;
    }
  }
  // Show in millions for other currencies
  if (balance >= 1e9) {
    return `${(balance / 1e9).toFixed(1)}B`;
  }
  if (balance >= 1e6) {
    return `${(balance / 1e6).toFixed(1)}M`;
  }
  return balance.toLocaleString();
};

export default function StatsPanel({
  totalProcessed,
  messagesPerSecond,
  tpsHistory = [],
  successRate,
  activeConnections,
  bankBalances,
  onRefreshLiquidity,
  lastLiquidityUpdate,
}: StatsPanelProps) {
  const [expandedCountry, setExpandedCountry] = useState<string | null>(null);

  // Group banks by country
  const groupedBanks = groupBanksByCountry(bankBalances);
  const countryOrder = ["MY", "SG", "TH", "ID", "PH", "VN"];

  // Calculate max balance per currency for relative visualization
  const maxBalanceByCurrency: Record<string, number> = {};
  bankBalances.forEach((bank) => {
    const curr = bank.currency || "MYR";
    if (!maxBalanceByCurrency[curr] || bank.balance > maxBalanceByCurrency[curr]) {
      maxBalanceByCurrency[curr] = bank.balance;
    }
  });
  const stats = [
    {
      label: "Total Processed",
      value: totalProcessed.toLocaleString(),
      icon: Activity,
      color: "text-primary",
    },
    {
      label: "Messages/Second",
      value: messagesPerSecond.toFixed(1),
      icon: Zap,
      color: "text-warning",
      hasSparkline: true,
    },
    {
      label: "Success Rate",
      value: `${successRate.toFixed(1)}%`,
      icon: TrendingUp,
      color: "text-success",
    },
    {
      label: "Active Connections",
      value: activeConnections.toString(),
      icon: Server,
      color: "text-primary",
    },
  ];

  return (
    <div className="space-y-4">
      {/* Main Stats Grid */}
      <div className="grid grid-cols-1 gap-2">
        {stats.map((stat, index) => (
          <motion.div
            key={stat.label}
            className="bg-surface border border-border rounded p-3"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: index * 0.05 }}
          >
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-text-muted uppercase font-bold tracking-tight">
                {stat.label}
              </span>
              <stat.icon className={`w-3 h-3 ${stat.color}`} />
            </div>
            <div className="flex items-end justify-between">
              <div className="text-xl font-bold text-text font-mono leading-none">
                {stat.value}
              </div>
              {stat.hasSparkline && (
                <div className="pb-0.5">
                   <Sparkline data={tpsHistory} color={stat.color} />
                </div>
              )}
            </div>
          </motion.div>
        ))}
      </div>

      {/* Bank Balances */}
      <div className="bg-surface border border-border rounded p-4 flex flex-col max-h-[400px]">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <DollarSign className="w-4 h-4 text-primary" />
            <h3 className="text-xs font-bold text-text uppercase tracking-widest">
              Bank Liquidity
            </h3>
          </div>
          <span className="text-[9px] font-mono text-text-muted">
            {bankBalances.length} banks
          </span>
        </div>

        {/* Countdown Timer */}
        <div className="mb-3 pb-2 border-b border-border/30">
          <div className="flex items-center justify-between mb-1">
            <span className="text-[9px] text-text-muted uppercase">Next Update</span>
          </div>
          <CountdownTimer
            lastUpdate={lastLiquidityUpdate}
            intervalMinutes={5}
            onRefresh={onRefreshLiquidity}
          />
        </div>

        {/* Scrollable Bank List */}
        <div className="flex-1 overflow-y-auto space-y-3 pr-1">
          {countryOrder.map((countryCode) => {
            const banks = groupedBanks[countryCode];
            if (!banks || banks.length === 0) return null;

            const countryName = banks[0].countryName || countryCode;
            const isExpanded = expandedCountry === countryCode || expandedCountry === null;
            const currency = banks[0].currency || "MYR";
            const maxBal = maxBalanceByCurrency[currency] || 1;

            return (
              <div key={countryCode} className="border-b border-border/20 pb-2 last:border-0">
                {/* Country Header */}
                <button
                  onClick={() => setExpandedCountry(expandedCountry === countryCode ? null : countryCode)}
                  className="w-full flex items-center justify-between py-1 hover:bg-primary/5 rounded transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <span className="text-sm">{countryFlags[countryCode] || "🏦"}</span>
                    <span className="text-[10px] font-bold text-text uppercase">
                      {countryName}
                    </span>
                    <span className="text-[9px] text-text-muted font-mono">
                      ({banks.length})
                    </span>
                  </div>
                  <span className="text-[9px] text-text-muted font-mono">
                    {currency}
                  </span>
                </button>

                {/* Bank List */}
                {isExpanded && (
                  <div className="mt-1 space-y-1 pl-5">
                    {banks.map((bank) => (
                      <div key={bank.bic} className="py-1">
                        <div className="flex items-center justify-between">
                          <span className="text-[10px] text-text truncate font-medium max-w-[100px]">
                            {bank.bankName}
                          </span>
                          <span className="text-[10px] font-mono text-success">
                            {formatBalance(bank.balance, bank.currency || "MYR")}
                          </span>
                        </div>
                        <LiquidityBar balance={bank.balance} maxBalance={maxBal} />
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* Last Updated Timestamp */}
        {lastLiquidityUpdate && (
          <div className="mt-3 pt-2 border-t border-border/30 text-[9px] text-text-muted text-center font-mono shrink-0">
            Last: {lastLiquidityUpdate.toLocaleTimeString([], { hour12: false })}
          </div>
        )}
      </div>
    </div>
  );
}
