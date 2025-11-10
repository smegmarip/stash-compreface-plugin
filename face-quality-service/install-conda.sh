#!/bin/bash
# Installation script for Quality Service in Conda environment

set -e

echo "Installing Quality Service dependencies in Conda environment..."

# Check if we're in a conda environment
if [ -z "$CONDA_DEFAULT_ENV" ]; then
    echo "WARNING: Not in a conda environment. Consider activating one first."
    echo "Example: conda create -n quality-service python=3.11"
    echo "         conda activate quality-service"
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

PYTHON_VERSION=$(python --version | awk '{print $2}')
echo "Detected Python version: $PYTHON_VERSION"
echo "Conda environment: ${CONDA_DEFAULT_ENV:-none}"

# Install conda packages first (faster and more reliable for compiled packages)
echo ""
echo "Installing packages via conda..."
conda install -y -c conda-forge \
    flask \
    opencv \
    numpy \
    pillow \
    requests

# Install dlib via pip (not available in conda-forge for all platforms)
echo ""
echo "Installing dlib via pip..."
pip install --no-cache-dir dlib>=19.24.0

# Verify installations
echo ""
echo "Verifying installations..."
python -c "import flask; print('✓ Flask:', flask.__version__)"
python -c "import cv2; print('✓ OpenCV:', cv2.__version__)"
python -c "import numpy; print('✓ NumPy:', numpy.__version__)"
python -c "import PIL; print('✓ Pillow:', PIL.__version__)"
python -c "import requests; print('✓ Requests:', requests.__version__)"
python -c "import dlib; print('✓ dlib:', dlib.__version__)"

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
echo "  python app.py"
echo ""
echo "To run tests:"
echo "  python test_service.py"
