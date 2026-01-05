"""
Liquidity Prediction Model for Nexus-Lite
This module implements a liquidity check model using Scikit-Learn for risk prediction.
"""

import json
import os
from typing import Dict, Tuple
import numpy as np
import logging
from datetime import datetime
from sklearn.linear_model import LogisticRegression
from sklearn.preprocessing import StandardScaler

logger = logging.getLogger(__name__)

class LiquidityModel:
    """
    Liquidity prediction model that checks bank balances and uses Scikit-Learn
    to predict liquidity risk based on transaction patterns.
    """
    
    def __init__(self, config_path: str = "/app/config/network.json"):
        """Initialize the liquidity model with bank balances from config."""
        self.bank_balances: Dict[str, float] = {}
        self.load_config(config_path)
        
        # Track transaction history
        self.transaction_history: list = []
        
        # ML Components
        self.clf = LogisticRegression(random_state=42)
        self.scaler = StandardScaler()
        self.is_fitted = False
        
        # Train initial model on synthetic data
        self._train_initial_model()
        
    def load_config(self, path: str):
        """Load bank balances from JSON configuration."""
        try:
            with open(path, 'r') as f:
                config = json.load(f)
                
            count = 0
            for bank in config.get('banks', []):
                # Only load banks that have an initial balance (sources)
                if 'initial_balance' in bank:
                    self.bank_balances[bank['id']] = bank['initial_balance']
                    count += 1
            
            logger.info(f"Loaded {count} bank balances from {path}")
            
        except Exception as e:
            logger.error(f"Failed to load config from {path}: {e}")
            # Fallback to empty (will cause rejections, which is safe)
            self.bank_balances = {}
    
    def _train_initial_model(self):
        """Generate synthetic data and train the initial risk model."""
        logger.info("Training initial Scikit-Learn liquidity risk model...")
        
        # Generate 1000 synthetic transactions
        # Features: [amount_ratio, hour_of_day]
        # Target: 1 (High Risk/Rejection), 0 (Low Risk/Approval)
        
        np.random.seed(42)
        n_samples = 1000
        
        # Feature 1: Amount Ratio (Transaction Amount / Balance)
        # Low risk: small ratios, High risk: ratios close to 1.0 or > 1.0
        amount_ratios = np.concatenate([
            np.random.beta(2, 10, 700),       # Safe transactions
            np.random.beta(5, 1, 300)         # Risky transactions
        ])
        
        # Feature 2: Hour of day (0-23)
        # Higher risk during non-business hours (e.g., 0-6 AM)
        hours = np.random.randint(0, 24, n_samples)
        
        X = np.column_stack((amount_ratios, hours))
        
        # Generate Targets (Logic: High ratio OR (Medium ratio AND odd hours) -> Risk)
        y = []
        for r, h in zip(amount_ratios, hours):
            risk_prob = r  # Base risk is the ratio
            if h < 6 or h > 20: # Night time penalty
                risk_prob += 0.2
            
            # If "probability" of failure is high, label as 1 (Risk)
            y.append(1 if risk_prob > 0.8 else 0)
            
        y = np.array(y)
        
        # Fit Scaler and Model
        self.scaler.fit(X)
        X_scaled = self.scaler.transform(X)
        self.clf.fit(X_scaled, y)
        self.is_fitted = True
        
        logger.info(f"Model trained on {n_samples} synthetic samples. Accuracy: {self.clf.score(X_scaled, y):.2f}")

    def get_all_balances(self) -> Dict[str, float]:
        """Return all current bank balances."""
        return self.bank_balances

    def check_liquidity(self, bank_id: str, transaction_amount: float, currency: str = "MYR") -> Tuple[bool, float, str, str]:
        """
        Check if a bank has sufficient liquidity for a transaction.
        
        Args:
            bank_id: Bank identifier (e.g., "MAYBANK", "CIMB")
            transaction_amount: Amount to transfer
            currency: Currency code (default: "MYR")
        
        Returns:
            Tuple of (approved, available_balance, error_code, error_message)
        """
        # Normalize bank_id to uppercase
        bank_id = bank_id.upper()
        
        # Check if bank exists
        if bank_id not in self.bank_balances:
            return (
                False,
                0.0,
                "AC04",
                f"Account closed: Bank '{bank_id}' not found"
            )
        
        current_balance = self.bank_balances[bank_id]
        
        # --- ML Risk Prediction (Non-blocking for Phase 3, but logged) ---
        risk_score = self.predict_liquidity_risk(bank_id, transaction_amount, current_balance)
        if risk_score > 0.8:
            logger.warning(f"High Liquidity Risk Detected for {bank_id}: {risk_score:.2f}")
        # -----------------------------------------------------------------

        # Deterministic Check (The Law)
        if current_balance < transaction_amount:
            return (
                False,
                current_balance,
                "AM04",
                f"Insufficient funds: Available {current_balance:.2f} {currency}, Risk Score: {risk_score:.2f}"
            )
        
        # Transaction approved - update balance and record history
        remaining_balance = current_balance - transaction_amount
        self.bank_balances[bank_id] = remaining_balance
        
        # Record transaction for future retraining
        self.transaction_history.append({
            "bank_id": bank_id,
            "amount": transaction_amount,
            "currency": currency,
            "previous_balance": current_balance,
            "remaining_balance": remaining_balance,
            "timestamp": datetime.now()
        })
        
        return (
            True,
            remaining_balance,
            "OK",
            f"Approved (Risk: {risk_score:.2f})"
        )
    
    def get_balance(self, bank_id: str) -> float:
        """Get the current balance for a bank."""
        bank_id = bank_id.upper()
        return self.bank_balances.get(bank_id, 0.0)
    
    def reset_balance(self, bank_id: str, amount: float) -> None:
        """Reset a bank's balance (useful for testing)."""
        bank_id = bank_id.upper()
        self.bank_balances[bank_id] = amount

    def credit_bank(self, bank_id: str, amount: float, currency: str = "MYR") -> Tuple[bool, float, str, str]:
        """
        Credit a bank when it receives funds from another bank.

        Args:
            bank_id: Bank identifier receiving funds
            amount: Amount to credit
            currency: Currency code

        Returns:
            Tuple of (success, new_balance, status_code, message)
        """
        bank_id = bank_id.upper()

        # Check if bank exists - if not, initialize it
        if bank_id not in self.bank_balances:
            self.bank_balances[bank_id] = 0.0
            logger.info(f"Initialized new bank {bank_id} with 0 balance")

        current_balance = self.bank_balances[bank_id]
        new_balance = current_balance + amount
        self.bank_balances[bank_id] = new_balance

        logger.debug(f"Credited {bank_id}: +{amount:.2f} {currency} (New balance: {new_balance:.2f})")

        return (
            True,
            new_balance,
            "OK",
            f"Credited {amount:.2f} {currency}"
        )
    
    def get_transaction_count(self, bank_id: str = None) -> int:
        """Get the count of processed transactions."""
        if bank_id:
            bank_id = bank_id.upper()
            return sum(1 for tx in self.transaction_history if tx["bank_id"] == bank_id)
        return len(self.transaction_history)
    
    def predict_liquidity_risk(self, bank_id: str, amount: float = 0.0, current_balance: float = None) -> float:
        """
        Predict liquidity risk score (0.0 to 1.0) using Scikit-Learn.
        
        Args:
            bank_id: Bank identifier
            amount: Transaction amount
            current_balance: Current balance (optional, fetches if None)
        
        Returns:
            Risk score (0.0 = safe, 1.0 = high risk)
        """
        if not self.is_fitted:
            return 0.5 # Default if model failed
            
        if current_balance is None:
            current_balance = self.get_balance(bank_id)
            
        if current_balance == 0:
            return 1.0
            
        # Prepare features: [amount_ratio, hour_of_day]
        amount_ratio = amount / current_balance if current_balance > 0 else 1.0
        hour = datetime.now().hour
        
        features = np.array([[amount_ratio, hour]])
        features_scaled = self.scaler.transform(features)
        
        # Predict probability of class 1 (High Risk)
        risk_prob = self.clf.predict_proba(features_scaled)[0][1]
        
        return float(risk_prob)


# Global model instance
_model = None


def get_model() -> LiquidityModel:
    """Get the global liquidity model instance."""
    global _model
    if _model is None:
        _model = LiquidityModel()
    return _model
