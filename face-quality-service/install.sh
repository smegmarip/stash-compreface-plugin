#!/bin/bash
# Installation script for Quality Service

set -e

echo "Installing Quality Service dependencies..."

# Check Python version
PYTHON_VERSION=$(python3 --version | awk '{print $2}')
echo "Detected Python version: $PYTHON_VERSION"

# Create virtual environment
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
source venv/bin/activate

# Upgrade pip and setuptools first
echo "Upgrading pip and setuptools..."
pip install --upgrade pip setuptools wheel

# Install dependencies
echo "Installing Python packages..."
pip install -r requirements.txt

# Check for model files
echo ""
echo "Checking for dlib model files..."

MODEL_DIR="../"
LANDMARKS_FILE="shape_predictor_68_face_landmarks.dat"
RESNET_FILE="dlib_face_recognition_resnet_model_v1.dat"

if [ -f "${MODEL_DIR}${LANDMARKS_FILE}" ]; then
    echo "✓ Found ${LANDMARKS_FILE}"
else
    echo "✗ Missing ${LANDMARKS_FILE}"
    echo "  Download from: http://dlib.net/files/shape_predictor_68_face_landmarks.dat.bz2"
    echo "  Extract to: ${MODEL_DIR}"
fi

if [ -f "${MODEL_DIR}${RESNET_FILE}" ]; then
    echo "✓ Found ${RESNET_FILE}"
else
    echo "✗ Missing ${RESNET_FILE}"
    echo "  Download from: http://dlib.net/files/dlib_face_recognition_resnet_model_v1.dat.bz2"
    echo "  Extract to: ${MODEL_DIR}"
fi

echo ""
echo "Installation complete!"
echo ""
echo "To start the service:"
echo "  source venv/bin/activate"
echo "  python app.py"
echo ""
echo "To run tests:"
echo "  source venv/bin/activate"
echo "  python test_service.py"
