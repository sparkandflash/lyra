#!/bin/bash

# Navigate to the directory containing the build script
cd "$(dirname "$0")"

BIN_NAME=$1
if [ -z "$BIN_NAME" ]; then
    echo "Usage: ./build.sh <binary_name>"
    exit 1
fi

BUILD_DIR="build"
mkdir -p "$BUILD_DIR"

BIN_DIR=".bin"
if [ ! -f "$BIN_DIR/ollama" ]; then
    echo "Setting up local embedding engine sidecar..."
    mkdir -p "$BIN_DIR/models"
    if ! command -v ollama &> /dev/null; then
        echo "system error: ollama is not installed on your system. Please install it first from https://ollama.com"
        exit 1
    fi
    echo "Symlinking system ollama into sidecar folder..."
    REAL_OLLAMA=$(readlink "$(command -v ollama)" || command -v ollama)
    ln -s "$REAL_OLLAMA" "$BIN_DIR/ollama"
    chmod +x "$BIN_DIR/ollama"
    
    echo "Pulling embedding model (this may take a moment)..."
    OLLAMA_MODELS="./$BIN_DIR/models" OLLAMA_HOST="127.0.0.1:11435" "./$BIN_DIR/ollama" serve &
    OLLAMA_PID=$!
    sleep 3
    OLLAMA_HOST="127.0.0.1:11435" "./$BIN_DIR/ollama" pull nomic-embed-text
    kill $OLLAMA_PID
    wait $OLLAMA_PID 2>/dev/null
    echo "Local embedding engine ready!"
fi

echo "Building $BIN_NAME binary..."
go build -o "$BUILD_DIR/$BIN_NAME" main.go

if [ $? -eq 0 ]; then
    echo "Successfully compiled standalone binary to $BUILD_DIR/$BIN_NAME"
else
    echo "Build failed!"
    exit 1
fi
