#!/bin/bash

# Wait for Redis nodes to be ready
sleep 10

# Create Redis cluster
echo "Creating Redis cluster..."
redis-cli --cluster create \
  redis_7000:7000 \
  redis_7001:7001 \
  redis_7002:7002 \
  redis_7100:7100 \
  redis_7101:7101 \
  redis_7102:7102 \
  --cluster-replicas 1 \
  --cluster-yes

echo "Redis cluster created successfully."