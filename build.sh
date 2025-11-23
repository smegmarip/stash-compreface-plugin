#!/bin/bash

# Stash Compreface Plugin - Build Script
# Builds RPC binary for multiple platforms

set -e

PLUGIN_NAME="stash-compreface-rpc"
BUILD_DIR="gorpc"
VERSION="2.0.0"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building Stash Compreface Plugin v${VERSION}${NC}"
echo ""

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}')
echo -e "Go version: ${YELLOW}${GO_VERSION}${NC}"

# Navigate to build directory
cd "${BUILD_DIR}"

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -f ${PLUGIN_NAME}*

# Build for Linux (Stash runs in Linux container)
echo -e "${GREEN}Building for Linux (amd64)...${NC}"
GOOS=linux GOARCH=amd64 TMPDIR=/Users/x/tmp GOTMPDIR=/Users/x/tmp go build -o ${PLUGIN_NAME} -ldflags "-s -w" .
if [ $? -eq 0 ]; then
    BINARY_SIZE=$(du -h ${PLUGIN_NAME} | awk '{print $1}')
    echo -e "${GREEN}✓ Built ${PLUGIN_NAME} (${BINARY_SIZE})${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo ""
echo -e "Binary:"
ls -lh ${PLUGIN_NAME} | awk '{print "  " $9 " (" $5 ")"}'
