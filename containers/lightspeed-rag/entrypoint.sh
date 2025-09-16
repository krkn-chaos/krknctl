#!/bin/bash
# Modified by Claude Sonnet 4
# Entrypoint script for krknctl Lightspeed RAG container

set -e

echo "Starting krknctl Lightspeed RAG service..."

# Create /dev/dri structure if GPU devices are mapped to different paths
if [ -c "/dev/card0" ] && [ -c "/dev/renderD128" ]; then
    echo "Creating /dev/dri structure for GPU access..."
    mkdir -p /dev/dri
    ln -sf /dev/card0 /dev/dri/card0
    ln -sf /dev/renderD128 /dev/dri/renderD128
    ls -la /dev/dri/
fi

# Check if we should use offline mode (env var set by krknctl)
USE_OFFLINE=${USE_OFFLINE:-"false"}

# Verify the model is available (should be pre-downloaded)
echo "Verifying Llama 3.2:1B model..."
MODEL_PATH="/app/models/Llama-3.2-1B-Instruct-Q4_K_M.gguf"
if [ ! -f "$MODEL_PATH" ]; then
    echo "WARNING: Model not found at $MODEL_PATH"
    echo "Service will attempt to download it on first use"
else
    echo "Model verified: $(basename $MODEL_PATH)"
fi

# Set environment variables for the krkn-lightspeed service
export MODEL_PATH="$MODEL_PATH"
export CONTAINER_ENV="true"

# Change to krkn-lightspeed directory
cd /app/krkn-lightspeed

# Handle documentation indexing based on online/offline mode
if [ "$USE_OFFLINE" = "true" ]; then
    echo "Using cached documentation (offline mode)"
    # Copy cached index to active location if exists
    if [ -d "/app/cached_docs" ]; then
        mkdir -p /app/docs_index
        cp -r /app/cached_docs/* /app/docs_index/ 2>/dev/null || echo "No cached docs found, will use embedded data"
    fi
else
    echo "Indexing live krknctl and krkn documentation..."
    # Note: The new service will handle documentation indexing internally
    echo "Documentation will be indexed by the service automatically"
fi

# Start the new FastAPI server from krkn-lightspeed
echo "Starting krkn-lightspeed FastAPI service on port 8080..."
exec python3 fastapi_server.py