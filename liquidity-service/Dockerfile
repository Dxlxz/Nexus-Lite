FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY liquidity-service/requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY liquidity-service/ .
COPY proto/ /proto/

# Generate Python protobuf code
RUN python -m grpc_tools.protoc -I/proto --python_out=. --grpc_python_out=. /proto/liquidity.proto

# Expose gRPC port
EXPOSE 50051

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import socket; s=socket.socket(); s.connect(('localhost', 50051))" || exit 1

# Run the service
CMD ["python", "main.py"]
