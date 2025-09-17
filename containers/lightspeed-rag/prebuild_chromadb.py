#!/usr/bin/env python3
"""
Pre-build ChromaDB with document embeddings during container build
This script runs the same logic as the RAG pipeline but saves the result for runtime reuse
"""

import sys
import os

# Add the krkn-lightspeed directory to Python path
sys.path.insert(0, '/app/krkn-lightspeed')

from utils.document_loader import clone_locally, load_and_split
from utils.embedding_config import get_embedding_model
from utils.build_collections import load_or_create_chroma_collection

def prebuild_chromadb():
    """Pre-build ChromaDB collection with document embeddings"""

    print("🔧 Pre-building ChromaDB collection during container build...")

    # Configuration (same as runtime)
    collection_name = "krkn-docs"
    persist_dir = "/app/docs_index"
    embedding_model = "qwen-small"

    # Document sources configuration
    repo_sources = [
        {
            "url": "https://github.com/krkn-chaos/website",
            "docs_path": "content/en/docs"
        },
        {
            "url": "https://github.com/krkn-chaos/krkn-hub",
            "docs_path": "."
        }
    ]

    # Chunking configuration (same as runtime)
    chunking_config = {
        "chunk_size": 1000,
        "chunk_overlap": 200,
    }

    try:
        # Step 1: Load and split documents
        print(f"📚 Loading documents from {len(repo_sources)} repositories + krknctl help...")
        all_splits = []

        # Clone repositories and load their docs
        for repo_info in repo_sources:
            print(f"Loading from: {repo_info['url']}")
            splits = clone_locally(
                repo_url=repo_info["url"],
                docs_path=repo_info["docs_path"],
                **chunking_config,
            )
            all_splits.extend(splits)
            print(f"Loaded {len(splits)} chunks from {repo_info['url']}")

        # Load krknctl help file
        print("Loading krknctl help file...")
        help_splits = load_and_split(["/app/krknctl_help.txt"], **chunking_config)
        all_splits.extend(help_splits)
        print(f"Loaded {len(help_splits)} chunks from krknctl help")

        print(f"Total document chunks: {len(all_splits)}")

        # Step 2: Get embedding model
        print(f"🔗 Loading embedding model: {embedding_model}")
        embedding_model_instance = get_embedding_model(embedding_model)

        # Step 3: Create ChromaDB collection
        print(f"💾 Building ChromaDB collection: {collection_name}")
        vector_store = load_or_create_chroma_collection(
            collection_name=collection_name,
            embedding_model=embedding_model_instance,
            all_splits=all_splits,
            persist_dir=persist_dir,
        )

        print("✅ ChromaDB pre-build completed successfully!")
        print(f"📍 Database saved to: {persist_dir}")

        # Verify the collection
        print("🔍 Verifying collection...")
        if hasattr(vector_store, '_collection'):
            count = vector_store._collection.count()
            print(f"✅ Collection contains {count} documents")

        return True

    except Exception as e:
        print(f"❌ Error during ChromaDB pre-build: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    success = prebuild_chromadb()
    sys.exit(0 if success else 1)