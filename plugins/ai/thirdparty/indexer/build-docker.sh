#!/bin/bash
# 构建 Indexer Docker 镜像

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

IMAGE_NAME="duanjunzi/rde-indexer"
TAG="${1:-latest}"

echo "Building $IMAGE_NAME:$TAG ..."

docker build -t "$IMAGE_NAME:$TAG" .

echo "Done! Image: $IMAGE_NAME:$TAG"
echo ""
echo "推送镜像:"
echo "  docker push $IMAGE_NAME:$TAG"
echo ""
echo "Run with docker-compose (recommended for RDE):"
echo "  docker-compose up -d"
echo ""
echo "Or standalone (multi-user mode):"
echo "  docker run -d \\"
echo "    -p 8081:8081 \\"
echo "    -e MULTI_USER=true \\"
echo "    -v /var/lib/rde/data/home:/data/homes:ro \\"
echo "    -v indexer_data:/data/index \\"
echo "    $IMAGE_NAME:$TAG"
echo ""
echo "Or standalone (single-user mode):"
echo "  docker run -d \\"
echo "    -p 8081:8081 \\"
echo "    -e MULTI_USER=false \\"
echo "    -v ~/Pictures:/data/images:ro \\"
echo "    -v ~/Documents:/data/documents:ro \\"
echo "    -v indexer_data:/data/index \\"
echo "    $IMAGE_NAME:$TAG"
