#!/bin/bash

echo "🚀 Starting the application..."

# Check if database is accessible
echo "🔍 Checking database connection..."
if command -v psql &> /dev/null; then
    if psql -U validator -d project-sem-1 -c "SELECT 1;" > /dev/null 2>&1; then
        echo "✅ Database connection successful"
    else
        echo "⚠️  Cannot connect to database, but starting server anyway..."
    fi
else
    echo "⚠️  PostgreSQL client not found, but starting server anyway..."
fi

# Build the application if not built
if [ ! -f bin/server ]; then
    echo "🔨 Building application..."
    go build -o bin/server main.go
fi

echo "🌐 Starting HTTP server on :8080"
echo "💡 Use Ctrl+C to stop the server"
echo "📊 Health check: curl http://localhost:8080/health"

# Run the application
./bin/server
