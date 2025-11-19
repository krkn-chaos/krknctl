# Local Testing Guide for RAG Scripts

This guide explains how to test and debug the RAG (Retrieval-Augmented Generation) scripts locally outside of the container environment.

## Prerequisites

1. **Python Virtual Environment**: Activate the existing venv with all dependencies:
   ```bash
   cd containers/assist-rag
   source venv/bin/activate
   ```

2. **Ollama Installation**: Make sure Ollama is installed and running locally:
   ```bash
   # Install Ollama (if not already installed)
   curl -fsSL https://ollama.com/install.sh | sh
   
   # Pull the required model
   ollama pull llama3.2:1b
   ```

## Local Testing Steps

### 1. Set up Local Directory Structure
```bash
# Create a local testing directory
mkdir -p ./local_rag
cd local_rag

# Copy the krknctl help file (optional, for local testing)
cp ../krknctl_help.txt ./
```

### 2. Build Documentation Index Locally
```bash
# Build documentation index with custom home directory
python3 ../index_docs.py --home ./local_rag --live-index

# Alternative: build to a specific output directory
python3 ../index_docs.py --home ./local_rag --output-dir ./custom_index --live-index
```

### 3. Run RAG Service Locally
```bash
# Run the RAG service with custom home directory
python3 ../rag_service.py --home ./local_rag --host 127.0.0.1 --port 8081

# The service will be available at http://127.0.0.1:8081
```

### 4. Test the Service
```bash
# Health check
curl http://127.0.0.1:8081/health

# Query example
curl -X POST http://127.0.0.1:8081/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How do I run a pod deletion scenario?", "max_results": 3}'
```

## Command Line Options

### rag_service.py
- `--home`: Base directory for RAG service files (default: `/app`)
- `--host`: Host to bind the service to (default: `0.0.0.0`)
- `--port`: Port to bind the service to (default: `8080`)

### index_docs.py
- `--home`: Base directory for RAG service files (default: `/app`)
- `--output-dir`: Output directory for the index (overrides default behavior)
- `--build-cached-index`: Build cached index (for container build time)
- `--live-index`: Build live index with fresh documentation

## Directory Structure

When using `--home ./local_rag`, the following structure is created:

```
local_rag/
├── krknctl_help.txt          # krknctl help content (optional)
├── docs_index/               # Default index location
│   ├── index.faiss          # FAISS vector index
│   ├── documents.json       # Document metadata
│   └── embeddings.npy       # Document embeddings
└── cached_docs/             # Cached index (if using --build-cached-index)
```

## Debugging Tips

1. **Check Logs**: Both scripts provide detailed logging to help debug issues
2. **Verify Ollama**: Make sure `ollama list` shows `llama3.2:1b` model
3. **Network Issues**: If scraping fails, check internet connectivity
4. **Port Conflicts**: Use different ports with `--port` if 8080 is busy

## Example: Complete Local Setup

```bash
# 1. Set up environment
cd containers/assist-rag
source venv/bin/activate
mkdir -p local_test && cd local_test

# 2. Build index
python3 ../index_docs.py --home . --live-index

# 3. Run service
python3 ../rag_service.py --home . --host 127.0.0.1 --port 8081

# 4. Test in another terminal
curl http://127.0.0.1:8081/health
```

This setup allows for easy debugging and testing without needing to rebuild containers.