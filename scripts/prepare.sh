#!/bin/bash

echo "🚀 Starting project preparation..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi
echo "✅ Go is installed: $(go version)"

# Install Go dependencies
echo "📦 Installing Go dependencies..."
go mod tidy
go mod download

echo "✅ Dependencies installed"

# Build the application
echo "🔨 Building application..."
mkdir -p bin
go build -o bin/server main.go

echo "🎉 Preparation completed!"
echo ""
echo "📋 Next steps:"
echo "  1. Run: ./scripts/run.sh"
echo "  2. Test: ./scripts/tests.sh 1"
