#!/bin/bash

echo "🧹 Cleaning up Kafka + Kerberos Testing Environment"
echo "===================================================="

# Stop and remove containers
echo "🛑 Stopping containers..."
docker-compose down -v

# Remove generated files
echo "🗂️  Removing generated files..."
rm -rf kerberos ssl logs
rm -f test_config.yaml perf_test.py

# Remove Docker images (optional)
read -p "🐳 Remove Docker images? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🗑️  Removing Docker images..."
    docker rmi $(docker images | grep -E "(confluentinc|gcavalcante8808|provectuslabs)" | awk '{print $3}') 2>/dev/null || echo "No images to remove"
fi

echo "✅ Cleanup complete!"
