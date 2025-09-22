#!/bin/bash
#
# Quick Kafka SSL Message Verification
# Proven working commands for ktranslate message verification
#

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BROKER="localhost:9093"
TOPIC="ktranslate-sasl-ssl-ultra-debug"
SSL_ARGS="-X security.protocol=SSL -X ssl.ca.location= -X ssl.endpoint.identification.algorithm=none -X enable.ssl.certificate.verification=false"

echo -e "${BLUE}=== Kafka SSL Message Verification Tool ===${NC}"
echo ""

# Function to show usage
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  list      - List topics and broker info"
    echo "  send      - Send a test message to ktranslate topic"
    echo "  read      - Read all messages from topic"
    echo "  follow    - Follow live messages (Ctrl+C to stop)"
    echo "  test      - Complete test: send message then read it"
    echo ""
    echo "Examples:"
    echo "  $0 list"
    echo "  $0 send"
    echo "  $0 read"
    echo "  $0 follow"
    echo "  $0 test"
}

# Function to list topics
list_topics() {
    echo -e "${YELLOW}Listing Kafka topics and broker info...${NC}"
    kcat -b $BROKER $SSL_ARGS -L
}

# Function to send test message
send_message() {
    local timestamp=$(date -Iseconds)
    local message='{"verification_test": true, "timestamp": "'$timestamp'", "src_addr": "192.168.1.100", "dst_addr": "10.0.0.200", "protocol": 6, "bytes": 1500}'
    
    echo -e "${YELLOW}Sending test message to topic: $TOPIC${NC}"
    echo "Message: $message"
    
    echo "$message" | kcat -b $BROKER $SSL_ARGS -t $TOPIC -P
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ Message sent successfully!${NC}"
    else
        echo -e "❌ Failed to send message"
        return 1
    fi
}

# Function to read all messages
read_messages() {
    echo -e "${YELLOW}Reading all messages from topic: $TOPIC${NC}"
    kcat -b $BROKER $SSL_ARGS -t $TOPIC -C -o beginning -e
}

# Function to follow live messages
follow_messages() {
    echo -e "${YELLOW}Following live messages from topic: $TOPIC${NC}"
    echo "Press Ctrl+C to stop..."
    kcat -b $BROKER $SSL_ARGS -t $TOPIC -C -o end
}

# Function to run complete test
run_test() {
    echo -e "${BLUE}Running complete verification test...${NC}"
    echo ""
    
    echo "Step 1: Check broker connectivity"
    if ! list_topics > /dev/null 2>&1; then
        echo -e "❌ Cannot connect to Kafka broker"
        return 1
    fi
    echo -e "${GREEN}✅ Broker connectivity OK${NC}"
    
    echo ""
    echo "Step 2: Send test message"
    if ! send_message; then
        return 1
    fi
    
    echo ""
    echo "Step 3: Read messages to verify"
    echo -e "${YELLOW}Messages in topic:${NC}"
    read_messages
    
    echo ""
    echo -e "${GREEN}🎉 Verification test completed successfully!${NC}"
}

# Main script logic
case "${1:-}" in
    "list")
        list_topics
        ;;
    "send")
        send_message
        ;;
    "read")
        read_messages
        ;;
    "follow")
        follow_messages
        ;;
    "test")
        run_test
        ;;
    "help"|"--help"|"-h")
        usage
        ;;
    "")
        usage
        ;;
    *)
        echo "Unknown command: $1"
        echo ""
        usage
        exit 1
        ;;
esac