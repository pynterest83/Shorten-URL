#!/bin/bash

# Ports for each instance
ports=(8081 8082 8083 8084 8085)

# Launch each instance in the background
for port in "${ports[@]}"; do
  echo "Starting instance on port $port..."
  go run app.go -port=$port &
done

# Wait for all background processes
wait
