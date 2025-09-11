#!/bin/bash

set -e

echo "🔍 KTranslate Environment Health Check"
echo "======================================"

# Check system requirements
echo "📋 System Requirements:"

# Docker
if command -v docker &> /dev/null; then
    echo "✅ Docker: $(docker --version | cut -d' ' -f3)"
else
    echo "❌ Docker: Not installed"
    exit 1
fi

# Docker Compose
if command -v docker-compose &> /dev/null; then
    echo "✅ Docker Compose: $(docker-compose --version | cut -d' ' -f3)"
else
    echo "❌ Docker Compose: Not installed"
    exit 1
fi

# Java (for keytool)
if command -v keytool &> /dev/null; then
    echo "✅ Java/Keytool: Available"
else
    echo "❌ Java/Keytool: Not installed (required for SSL certificates)"
    echo "   Install with: sudo apt-get install default-jdk"
    exit 1
fi

# OpenSSL
if command -v openssl &> /dev/null; then
    echo "✅ OpenSSL: $(openssl version | cut -d' ' -f2)"
else
    echo "❌ OpenSSL: Not installed"
    exit 1
fi

# Python3 (for performance tests)
if command -v python3 &> /dev/null; then
    echo "✅ Python3: $(python3 --version | cut -d' ' -f2)"
else
    echo "⚠️  Python3: Not installed (performance tests will be skipped)"
fi

echo ""
echo "🐳 Docker Environment:"

# Docker daemon
if docker info &> /dev/null; then
    echo "✅ Docker daemon: Running"
else
    echo "❌ Docker daemon: Not running"
    echo "   Start with: sudo systemctl start docker"
    exit 1
fi

# Available memory
total_mem=$(free -m | awk '/^Mem:/{print $2}')
available_mem=$(free -m | awk '/^Mem:/{print $7}')
echo "📊 System Memory: ${available_mem}MB available / ${total_mem}MB total"

if [ "$available_mem" -lt 2048 ]; then
    echo "⚠️  Warning: Less than 2GB available memory. Environment may be slow."
fi

# Available disk space
available_disk=$(df -h . | tail -1 | awk '{print $4}')
echo "💾 Available Disk: $available_disk"

echo ""
echo "🔧 KTranslate Build Check:"

# Check if ktranslate binary exists
if [ -f "../ktranslate" ]; then
    echo "✅ KTranslate binary: Found"
else
    echo "⚠️  KTranslate binary: Not found"
    echo "   Building now..."
    cd .. && make && cd testing
    if [ -f "../ktranslate" ]; then
        echo "✅ KTranslate binary: Built successfully"
    else
        echo "❌ KTranslate binary: Build failed"
        exit 1
    fi
fi

# Check Go version (if available)
if command -v go &> /dev/null; then
    echo "✅ Go version: $(go version | awk '{print $3}')"
else
    echo "⚠️  Go: Not installed (required for building from source)"
fi

echo ""
echo "🌐 Network Connectivity:"

# Check if ports are available
check_port() {
    local port=$1
    local service=$2
    if netstat -tuln 2>/dev/null | grep -q ":$port "; then
        echo "❌ Port $port ($service): Already in use"
        return 1
    else
        echo "✅ Port $port ($service): Available"
        return 0
    fi
}

port_issues=0
check_port 88 "Kerberos KDC" || ((port_issues++))
check_port 2181 "Zookeeper" || ((port_issues++))
check_port 9092 "Kafka PLAINTEXT" || ((port_issues++))
check_port 9094 "Kafka SASL_SSL" || ((port_issues++))
check_port 8080 "Kafka UI" || ((port_issues++))

if [ $port_issues -gt 0 ]; then
    echo "⚠️  Warning: $port_issues port(s) in use. You may need to stop other services."
    echo "   Check with: sudo netstat -tulpn | grep -E ':(88|2181|9092|9094|8080)'"
fi

echo ""
echo "📁 File Permissions:"

# Check directory permissions
if [ -w . ]; then
    echo "✅ Testing directory: Writable"
else
    echo "❌ Testing directory: Not writable"
    exit 1
fi

# Check if files exist
files_to_check=("setup.sh" "test.sh" "cleanup.sh" "docker-compose.yml")
for file in "${files_to_check[@]}"; do
    if [ -f "$file" ]; then
        echo "✅ $file: Found"
    else
        echo "❌ $file: Missing"
        exit 1
    fi
done

echo ""
echo "🎯 Pre-flight Summary:"
echo "======================"

if [ $port_issues -eq 0 ] && [ "$available_mem" -gt 1024 ]; then
    echo "✅ Environment ready for testing!"
    echo ""
    echo "🚀 Next steps:"
    echo "   ./setup.sh    # Set up the test environment (3-5 minutes)"
    echo "   ./test.sh     # Run comprehensive tests"
    echo "   ./cleanup.sh  # Clean up when done"
else
    echo "⚠️  Environment has warnings but should work"
    echo ""
    echo "🚀 You can still proceed with:"
    echo "   ./setup.sh    # Set up the test environment"
fi

echo ""
echo "📖 For detailed information, see README.md"
