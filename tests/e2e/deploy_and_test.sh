#!/bin/bash

# E2E Testing - Deploy and Test Script
# Deploys the Compreface plugin to local Stash instance and runs E2E tests

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Stash Compreface Plugin - E2E Testing${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Configuration
STASH_URL="http://localhost:9999"
STASH_PLUGINS_DIR="/Users/x/dev/resources/docker/stash/config/plugins"
PLUGIN_ID="compreface-rpc"  # ID from yml filename
PLUGIN_DIR_NAME="stash-compreface"  # Directory name
PLUGIN_SOURCE_DIR="/Users/x/dev/resources/repo/stash-compreface-plugin"
BINARY_NAME="stash-compreface-rpc"

# Step 1: Build Linux binary
echo -e "${BLUE}Step 1: Building Linux binary...${NC}"
cd "${PLUGIN_SOURCE_DIR}"
./build.sh > /dev/null 2>&1

if [ ! -f "gorpc/${BINARY_NAME}" ]; then
    echo -e "${RED}✗ Binary not found: gorpc/${BINARY_NAME}${NC}"
    exit 1
fi

BINARY_SIZE=$(du -h "gorpc/${BINARY_NAME}" | awk '{print $1}')
echo -e "${GREEN}✓ Binary built: ${BINARY_NAME} (${BINARY_SIZE})${NC}"
echo ""

# Step 2: Deploy plugin to Stash
echo -e "${BLUE}Step 2: Deploying plugin to Stash...${NC}"

# Create plugin directory if it doesn't exist
PLUGIN_DEST_DIR="${STASH_PLUGINS_DIR}/${PLUGIN_DIR_NAME}"
mkdir -p "${PLUGIN_DEST_DIR}/gorpc"

# Copy files using rsync (only essential files)
rsync -av --delete \
    --include='compreface-rpc.yml' \
    --include='gorpc/***' \
    --exclude='gorpc/tests/***' \
    --exclude='samples/***' \
    --exclude='docs/***' \
    --exclude='tests/***' \
    --exclude='.git/***' \
    --exclude='*' \
    "${PLUGIN_SOURCE_DIR}/" \
    "${PLUGIN_DEST_DIR}/"

# Ensure binary is executable
chmod +x "${PLUGIN_DEST_DIR}/gorpc/${BINARY_NAME}"

echo -e "${GREEN}✓ Plugin deployed to: ${PLUGIN_DEST_DIR}${NC}"
echo ""

# Step 3: Reload Stash plugins
echo -e "${BLUE}Step 3: Reloading Stash plugins...${NC}"

# Trigger plugin reload via GraphQL mutation
RELOAD_RESULT=$(curl -s "${STASH_URL}/graphql" \
    -H "Content-Type: application/json" \
    -d '{"query":"mutation{reloadPlugins}"}')

echo -e "${GREEN}✓ Plugins reloaded${NC}"
echo ""

# Step 4: Verify plugin is loaded
echo -e "${BLUE}Step 4: Verifying plugin registration...${NC}"

sleep 2  # Give Stash time to load the plugin

PLUGIN_INFO=$(curl -s "${STASH_URL}/graphql" \
    -H "Content-Type: application/json" \
    -d '{"query":"query{plugins{id name version enabled}}"}' | \
    python3 -m json.tool)

if echo "$PLUGIN_INFO" | grep -q "\"id\": \"${PLUGIN_ID}\""; then
    PLUGIN_NAME=$(echo "$PLUGIN_INFO" | grep -A 2 "\"id\": \"${PLUGIN_ID}\"" | grep "name" | cut -d'"' -f4)
    PLUGIN_VERSION=$(echo "$PLUGIN_INFO" | grep -A 3 "\"id\": \"${PLUGIN_ID}\"" | grep "version" | cut -d'"' -f4)
    PLUGIN_ENABLED=$(echo "$PLUGIN_INFO" | grep -A 4 "\"id\": \"${PLUGIN_ID}\"" | grep "enabled" | cut -d':' -f2 | tr -d ' ,')

    echo -e "${GREEN}✓ Plugin registered:${NC}"
    echo -e "  Name: ${PLUGIN_NAME}"
    echo -e "  Version: ${PLUGIN_VERSION}"
    echo -e "  Enabled: ${PLUGIN_ENABLED}"
else
    echo -e "${RED}✗ Plugin not found in Stash${NC}"
    echo ""
    echo "Available plugins:"
    echo "$PLUGIN_INFO"
    exit 1
fi

echo ""

# Step 5: Verify plugin tasks are available
echo -e "${BLUE}Step 5: Checking available plugin tasks...${NC}"

TASKS_RESULT=$(curl -s "${STASH_URL}/graphql" \
    -H "Content-Type: application/json" \
    -d "{\"query\":\"query{plugins{id name tasks{name description}}}\"}" | \
    python3 -m json.tool)

TASK_COUNT=$(echo "$TASKS_RESULT" | grep -A 100 "\"id\": \"${PLUGIN_ID}\"" | grep '"name":' | wc -l | tr -d ' ')

if [ "$TASK_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ Found ${TASK_COUNT} plugin tasks${NC}"
    echo ""
    echo "Available tasks:"
    echo "$TASKS_RESULT" | grep -A 100 "\"id\": \"${PLUGIN_ID}\"" | grep -A 1 '"name":' | grep -v "^--$" | head -20
else
    echo -e "${YELLOW}⚠ No tasks found for plugin${NC}"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Check Stash UI: ${STASH_URL}/settings?tab=tasks"
echo "  2. Run E2E tests: cd tests/e2e && ./run_e2e_tests.sh"
echo "  3. Check plugin logs in Stash"
echo ""
