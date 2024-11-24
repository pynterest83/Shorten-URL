#!/bin/bash

# Ports for each instance
ports=(8080 8081 8082 8083 8084)

# Launch each instance in the background
for port in "${ports[@]}"; do
  echo "Starting instance on port $port..."
  go run . -port=$port &
done

# Wait for all background processes
wait