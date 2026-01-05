"""
gRPC Server for Liquidity Check Service
This module implements the gRPC server that exposes the liquidity prediction model.
"""

import grpc
from concurrent import futures
import logging
import time
import sys
import os

# Add parent directory to path for proto imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

# Import generated protobuf code
# Note: These will be generated from proto/liquidity.proto
# For now, we'll create the stub classes
import liquidity_pb2
import liquidity_pb2_grpc

from model import LiquidityModel
from health import start_health_server, mark_service_ready, mark_model_loaded

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class LiquidityCheckServiceImpl(liquidity_pb2_grpc.LiquidityCheckServiceServicer):
    """
    Implementation of the LiquidityCheckService gRPC service.
    """
    
    def __init__(self, config_path: str):
        """Initialize the service with the liquidity model."""
        self.model = LiquidityModel(config_path)
        mark_model_loaded(True)
        logger.info(f"LiquidityCheckService initialized with config: {config_path}")
    
    def CheckLiquidity(self, request, context):
        """
        Check if a bank has sufficient liquidity for a transaction.
        
        Args:
            request: LiquidityCheckRequest with bank_id, transaction_amount, currency
            context: gRPC context
        
        Returns:
            LiquidityCheckResponse with approval status, balance, and error codes
        """
        start_time = time.time()
        
        logger.info(
            f"Liquidity check request - Bank: {request.bank_id}, "
            f"Amount: {request.transaction_amount} {request.currency}"
        )
        
        # Call the liquidity model
        approved, available_balance, error_code, error_message = self.model.check_liquidity(
            bank_id=request.bank_id,
            transaction_amount=request.transaction_amount,
            currency=request.currency
        )
        
        latency_ms = (time.time() - start_time) * 1000
        
        logger.info(
            f"Liquidity check result - Approved: {approved}, "
            f"Balance: {available_balance:.2f}, "
            f"ErrorCode: {error_code}, "
            f"Latency: {latency_ms:.2f}ms"
        )
        
        # Build response
        response = liquidity_pb2.LiquidityCheckResponse(
            approved=approved,
            available_balance=available_balance,
            error_code=error_code,
            error_message=error_message
        )
        
        return response

    def CreditBank(self, request, context):
        """
        Credit a bank when it receives funds from a transaction.

        Args:
            request: CreditBankRequest with bank_id, amount, currency
            context: gRPC context

        Returns:
            CreditBankResponse with success status and new balance
        """
        start_time = time.time()

        logger.info(
            f"Credit bank request - Bank: {request.bank_id}, "
            f"Amount: {request.amount} {request.currency}"
        )

        # Call the model's credit method
        success, new_balance, status_code, message = self.model.credit_bank(
            bank_id=request.bank_id,
            amount=request.amount,
            currency=request.currency
        )

        latency_ms = (time.time() - start_time) * 1000

        logger.info(
            f"Credit bank result - Success: {success}, "
            f"New Balance: {new_balance:.2f}, "
            f"Latency: {latency_ms:.2f}ms"
        )

        response = liquidity_pb2.CreditBankResponse(
            success=success,
            new_balance=new_balance,
            status_code=status_code,
            message=message
        )

        return response

    def GetBalances(self, request, context):
        """
        Get current balances for all banks.
        """
        balances_dict = self.model.get_all_balances()

        balances = []
        for bank_id, balance in balances_dict.items():
            balances.append(liquidity_pb2.BankBalance(
                bank_id=bank_id,
                balance=balance,
                currency="MYR"  # Default currency for this demo
            ))

        return liquidity_pb2.GetBalancesResponse(balances=balances)


def serve(port: int = 50051, max_workers: int = 10, config_path: str = "/app/config/network.json"):
    """
    Start the gRPC server.

    Args:
        port: Port to listen on (default: 50051)
        max_workers: Maximum number of worker threads
        config_path: Path to configuration file
    """
    # Start health check HTTP server
    health_port = int(os.environ.get('HEALTH_PORT', '8080'))
    start_health_server(health_port)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))

    # Add the service to the server
    liquidity_pb2_grpc.add_LiquidityCheckServiceServicer_to_server(
        LiquidityCheckServiceImpl(config_path),
        server
    )

    # Bind the server to the port
    listen_addr = f'[::]:{port}'
    server.add_insecure_port(listen_addr)

    logger.info(f"Starting gRPC server on {listen_addr}")
    logger.info(f"Max workers: {max_workers}")

    # Start the server
    server.start()
    mark_service_ready(True)

    logger.info("Liquidity Check Service is running...")

    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Shutting down server...")
        mark_service_ready(False)
        server.stop(grace=5)
        logger.info("Server stopped")


if __name__ == '__main__':
    import argparse
    
    parser = argparse.ArgumentParser(description='Liquidity Check gRPC Server')
    parser.add_argument(
        '--port',
        type=int,
        default=50051,
        help='Port to listen on (default: 50051)'
    )
    parser.add_argument(
        '--workers',
        type=int,
        default=10,
        help='Maximum number of worker threads (default: 10)'
    )
    
    args = parser.parse_args()
    
    serve(port=args.port, max_workers=args.workers)
