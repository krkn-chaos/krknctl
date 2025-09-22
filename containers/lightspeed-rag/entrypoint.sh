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

# Verify the model is available (should be pre-downloaded)
echo "Verifying Llama 3.2:1B model..."
MODEL_PATH="/app/models/Llama-3.2-3B-Instruct-Q4_K_M.gguf"
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

# Documentation handling
echo "Documentation will be indexed by krkn-lightspeed service automatically"

# Start the new FastAPI server from krkn-lightspeed
echo "Starting krkn-lightspeed FastAPI service on port 8080..."
exec python3 fastapi_server.py
