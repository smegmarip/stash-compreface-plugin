FROM python:3.11-slim

# Install system dependencies for OpenCV and dlib
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    libopencv-dev \
    python3-opencv \
    libboost-all-dev \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy requirements and install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY app.py .

# Create temp directory
RUN mkdir -p /tmp/quality_service

# Expose port
EXPOSE 6001

# Environment variables
ENV QUALITY_SERVICE_PORT=6001
ENV QUALITY_SERVICE_TEMP=/tmp/quality_service

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import requests; requests.get('http://localhost:6001/health')"

# Run the application
CMD ["python", "app.py"]
