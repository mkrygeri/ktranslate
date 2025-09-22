#!/bin/bash
#
# Kafka Message Verifier Setup
# Installs required Python dependencies for the message verifier
#

echo "=== Kafka Message Verifier Setup ==="
echo "Installing Python kafka-python library..."

# Check if pip3 is available
if ! command -v pip3 &> /dev/null; then
    echo "ERROR: pip3 not found. Please install python3-pip:"
    echo "  sudo apt-get update && sudo apt-get install python3-pip"
    exit 1
fi

# Install kafka-python
echo "Installing kafka-python..."
pip3 install kafka-python

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Usage examples:"
echo ""
echo "1. PLAINTEXT connection:"
echo "   ./kafka-message-verifier.py --topic test-topic --brokers localhost:9092 --security PLAINTEXT"
echo ""
echo "2. SSL connection:"
echo "   ./kafka-message-verifier.py --topic ktranslate-ssl-ultra-debug --brokers localhost:9093 --security SSL"
echo ""
echo "3. SASL_SSL connection (requires Kerberos setup):"
echo "   ./kafka-message-verifier.py --topic ktranslate-comprehensive-debug --brokers localhost:9093 --security SASL_SSL"
echo ""
echo "4. Follow mode (continuous consumption):"
echo "   ./kafka-message-verifier.py --topic test-topic --follow --security SSL"
echo ""
echo "5. Compact output (no metadata):"
echo "   ./kafka-message-verifier.py --topic test-topic --compact --security SSL"
echo ""
echo "6. Limit message count:"
echo "   ./kafka-message-verifier.py --topic test-topic --count 10 --security SSL"
echo ""