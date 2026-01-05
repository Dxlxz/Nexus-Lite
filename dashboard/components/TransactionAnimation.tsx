"use client";

import { motion, AnimatePresence } from "framer-motion";
import { Transaction } from "@/lib/types";

interface TransactionAnimationProps {
  transactions: Transaction[];
}

export default function TransactionAnimation({
  transactions,
}: TransactionAnimationProps) {
  const getStatusColor = (status: string) => {
    switch (status) {
      case "approved":
        return "#00ff88";
      case "rejected":
        return "#ff0055";
      case "pending":
        return "#ffcc00";
      default:
        return "#00f0ff";
    }
  };

  return (
    <div className="relative w-full h-full overflow-hidden">
      <AnimatePresence>
        {transactions.map((tx) => (
          <motion.div
            key={tx.id}
            className="absolute"
            initial={{ opacity: 0, scale: 0 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0 }}
            transition={{ duration: 0.3 }}
          >
            {/* Transaction Particle */}
            <motion.div
              className="w-3 h-3 rounded-full"
              style={{
                backgroundColor: getStatusColor(tx.status),
                boxShadow: `0 0 10px ${getStatusColor(tx.status)}`,
              }}
              animate={{
                x: [0, 100],
                y: [0, -50],
              }}
              transition={{
                duration: 2,
                ease: "easeInOut",
              }}
            >
              {/* Trail Effect */}
              <motion.div
                className="absolute inset-0 rounded-full"
                style={{
                  backgroundColor: getStatusColor(tx.status),
                  opacity: 0.3,
                }}
                animate={{
                  scale: [1, 2, 3],
                  opacity: [0.5, 0.3, 0],
                }}
                transition={{
                  duration: 2,
                  ease: "easeOut",
                }}
              />
            </motion.div>

            {/* Transaction Label */}
            <motion.div
              className="absolute top-4 left-1/2 -translate-x-1/2 bg-panel-bg/90 border border-primary/20 rounded px-2 py-1 text-xs whitespace-nowrap backdrop-blur-sm"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.2 }}
            >
              <span className="text-text-muted">{tx.currency}</span>{" "}
              <span className="text-primary font-mono">
                {tx.amount.toLocaleString()}
              </span>
            </motion.div>
          </motion.div>
        ))}
      </AnimatePresence>

      {/* Floating Particles Background */}
      <div className="absolute inset-0 pointer-events-none">
        {[...Array(20)].map((_, i) => (
          <motion.div
            key={i}
            className="absolute w-1 h-1 rounded-full bg-primary/20"
            initial={{
              x: Math.random() * 100 + "%",
              y: Math.random() * 100 + "%",
            }}
            animate={{
              y: [null, -20],
              opacity: [0.2, 0.5, 0.2],
            }}
            transition={{
              duration: 3 + Math.random() * 2,
              repeat: Infinity,
              ease: "easeInOut",
              delay: Math.random() * 2,
            }}
          />
        ))}
      </div>
    </div>
  );
}
