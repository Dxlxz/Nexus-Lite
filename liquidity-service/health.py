"""
Health check endpoints for Liquidity Service
Provides HTTP endpoints for Docker healthchecks and monitoring
"""

import logging
import threading
import json
import os
from http.server import BaseHTTPRequestHandler, HTTPServer
from datetime import datetime

logger = logging.getLogger(__name__)


class HealthCheckHandler(BaseHTTPRequestHandler):
    """HTTP handler for health check endpoints"""

    # Class variables to store service status
    service_ready = False
    model_loaded = False
    start_time = datetime.now()

    def log_message(self, format, *args):
        """Override to use our logger instead of stderr"""
        logger.debug("%s - %s" % (self.address_string(), format % args))

    def do_GET(self):
        """Handle GET requests for /health and /ready"""

        if self.path == '/health':
            self.handle_health()
        elif self.path == '/ready':
            self.handle_ready()
        else:
            self.send_error(404, "Not Found")

    def handle_health(self):
        """Liveness probe - service is running"""
        uptime = (datetime.now() - self.start_time).total_seconds()

        status = {
            "status": "healthy",
            "service": "liquidity-service",
            "timestamp": datetime.now().isoformat(),
            "uptime_seconds": uptime
        }

        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(status).encode())

    def handle_ready(self):
        """Readiness probe - service can accept traffic"""
        ready = self.service_ready and self.model_loaded

        status = {
            "ready": ready,
            "service": "liquidity-service",
            "timestamp": datetime.now().isoformat(),
            "model_loaded": self.model_loaded,
            "grpc_server_ready": self.service_ready
        }

        if ready:
            self.send_response(200)
        else:
            self.send_response(503)

        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(status).encode())


def start_health_server(port=8080):
    """Start health check HTTP server in background thread"""
    server = HTTPServer(('0.0.0.0', port), HealthCheckHandler)

    def serve():
        logger.info(f"Health check server starting on port {port}")
        server.serve_forever()

    thread = threading.Thread(target=serve, daemon=True)
    thread.start()
    logger.info(f"Health check endpoints available: http://localhost:{port}/health and /ready")

    return server


def mark_service_ready(ready=True):
    """Mark gRPC service as ready"""
    HealthCheckHandler.service_ready = ready
    logger.info(f"Service readiness set to: {ready}")


def mark_model_loaded(loaded=True):
    """Mark ML model as loaded"""
    HealthCheckHandler.model_loaded = loaded
    logger.info(f"Model loaded status set to: {loaded}")
