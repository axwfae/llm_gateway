#!/bin/bash
set -e

echo "Building LLM Gateway Docker image..."
#cd "$(dirname "$0")"

podman  build -t llm_gateway:latest .
echo "Build complete!"
echo ""
echo "To run the container:"
echo "  docker run -d -p 18869:18869 -p 18866:18866 -v \$(pwd)/config:/app/config llm_gateway:latest"
echo ""
echo "Or use docker-compose:"
echo "  docker-compose up -d"
