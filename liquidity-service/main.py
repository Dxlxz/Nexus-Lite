"""
Main entry point for the Liquidity Check Service
This module provides the main entry point for running the gRPC server.
"""

import logging
import sys
import os

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def main():
    """Main entry point for the Liquidity Check Service."""
    logger.info("=" * 60)
    logger.info("Nexus-Lite: Liquidity Check Service")
    logger.info("Phase 3 - Liquidity Prediction Integration")
    logger.info("=" * 60)
    
    # Import and start the server
    try:
        from server import serve
        
        # Default configuration
        port = int(os.environ.get('LIQUIDITY_SERVICE_PORT', '50051'))
        max_workers = int(os.environ.get('LIQUIDITY_SERVICE_WORKERS', '10'))
        config_path = os.environ.get('CONFIG_PATH', '/app/config/network.json')

        logger.info(f"Configuration:")
        logger.info(f"  - Port: {port}")
        logger.info(f"  - Max Workers: {max_workers}")
        logger.info(f"  - Config Path: {config_path}")
        logger.info(f"  - Protocol: gRPC (HTTP/2)")
        logger.info(f"  - Serialization: Protocol Buffers")
        logger.info("")

        # Start the server
        serve(port=port, max_workers=max_workers, config_path=config_path)
        
    except KeyboardInterrupt:
        logger.info("Service stopped by user")
        sys.exit(0)
    except Exception as e:
        logger.error(f"Failed to start service: {e}", exc_info=True)
        sys.exit(1)


if __name__ == '__main__':
    main()
