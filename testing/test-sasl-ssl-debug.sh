#!/bin/bash

# ktranslate SASL_SSL Debug Test Script
# This script tests the complete SASL_SSL configuration with maximum debugging

set -e

echo "🔍 ktranslate SASL_SSL Comprehensive Debug Test"
echo "================================================"

# Check if required files exist
echo "📁 Checking required files..."
if [[ ! -f "/home/mikek/ktranslate/testing/krb5.conf" ]]; then
    echo "❌ Missing Kerberos config: /home/mikek/ktranslate/testing/krb5.conf"
    exit 1
fi

if [[ ! -f "/home/mikek/ktranslate/testing/ktranslate-debug.keytab" ]]; then
    echo "❌ Missing keytab: /home/mikek/ktranslate/testing/ktranslate-debug.keytab"
    exit 1
fi

if [[ ! -f "/home/mikek/ktranslate/testing/ssl-certs/ca-cert" ]]; then
    echo "❌ Missing SSL CA cert: /home/mikek/ktranslate/testing/ssl-certs/ca-cert"
    exit 1
fi

echo "✅ All required files present"

# Check infrastructure
echo "🐳 Checking Docker infrastructure..."
if ! docker ps | grep -q "ktranslate-kdc"; then
    echo "❌ Kerberos KDC container not running"
    exit 1
fi

if ! docker ps | grep -q "kafka.*9093"; then
    echo "⚠️  Warning: No Kafka broker found on port 9093"
fi

echo "✅ Infrastructure check complete"

# Test Kerberos authentication
echo "🎫 Testing Kerberos authentication..."
export KRB5_CONFIG=/home/mikek/ktranslate/testing/krb5.conf

# Clear any existing tickets
kdestroy 2>/dev/null || true

# Get new ticket
if kinit -kt /home/mikek/ktranslate/testing/ktranslate-debug.keytab ktranslate@EXAMPLE.COM; then
    echo "✅ Kerberos authentication successful"
    klist
else
    echo "❌ Kerberos authentication failed"
    exit 1
fi

# Set up environment for maximum debugging
echo "🔧 Setting up debug environment..."
export GODEBUG=tls=1
export KRB5_TRACE=/dev/stderr

echo "🚀 Ready to run ktranslate with SASL_SSL and maximum debugging!"
echo ""
echo "Run this command:"
echo "cd /home/mikek/ktranslate"
echo "export KRB5_CONFIG=/home/mikek/ktranslate/testing/krb5.conf"
echo "export GODEBUG=tls=1"
echo "export KRB5_TRACE=/dev/stderr"
echo "./ktranslate \\"
echo "  -config=/home/mikek/ktranslate/testing/ktranslate-comprehensive-debug.yaml \\"
echo "  -log_level=debug \\"
echo "  -stdout \\"
echo "  -nf.source=netflow5 \\"
echo "  -nf.port=9995"