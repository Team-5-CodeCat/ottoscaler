#!/bin/bash

# Redis Event Test Script
# Sends a test scale_up event to Redis Streams

set -e

# Configuration (can be overridden by environment variables)
REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_PORT=${REDIS_PORT:-6379}
REDIS_STREAM=${REDIS_STREAM:-otto:scale:events}

echo "🧪 Redis Event Test Script"
echo "📡 Target: redis://${REDIS_HOST}:${REDIS_PORT}"
echo "📊 Stream: ${REDIS_STREAM}"
echo ""

# Check if redis-cli is available
if ! command -v redis-cli &> /dev/null; then
    echo "❌ redis-cli is not installed. Please install Redis client."
    echo "Ubuntu/Debian: sudo apt-get install redis-tools"
    echo "macOS: brew install redis"
    exit 1
fi

# Test Redis connection
echo "🔌 Testing Redis connection..."
if ! redis-cli -h ${REDIS_HOST} -p ${REDIS_PORT} ping &> /dev/null; then
    echo "❌ Cannot connect to Redis at ${REDIS_HOST}:${REDIS_PORT}"
    echo "Make sure Redis server is running"
    exit 1
fi
echo "✅ Redis connection successful"
echo ""

# Generate unique task ID
TASK_ID="task-$(date +%s)-$$"
EVENT_ID=$(date +%s%3N)

echo "📤 Sending test scale_up event..."
echo "   Task ID: ${TASK_ID}"
echo "   Pod Count: 2"

# Send scale_up event
redis-cli -h ${REDIS_HOST} -p ${REDIS_PORT} XADD ${REDIS_STREAM} \* \
    type scale_up \
    pod_count 2 \
    task_id ${TASK_ID} \
    timestamp $(date +%s) \
    test_event true

echo "✅ Event sent successfully!"
echo ""

# Show recent events in the stream
echo "📋 Recent events in stream:"
redis-cli -h ${REDIS_HOST} -p ${REDIS_PORT} XREAD COUNT 3 STREAMS ${REDIS_STREAM} 0

echo ""
echo "🎯 Test complete! Monitor your Ottoscaler logs to see event processing."