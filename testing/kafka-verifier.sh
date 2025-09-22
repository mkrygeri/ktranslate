#!/bin/bash
#
# Simple Kafka Message Verifier using kafkacat/kcat
# Alternative to Python-based verifier that uses system tools
#

set -e

# Configuration
TOPIC=""
BROKERS="localhost:9092"
SECURITY="PLAINTEXT"
SSL_PORT="9093"
COUNT=""
TIMEOUT=30
FOLLOW=false
COMPACT=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

usage() {
    echo "Kafka Message Verifier - Simple bash/kafkacat version"
    echo ""
    echo "Usage: $0 --topic TOPIC [OPTIONS]"
    echo ""
    echo "Required:"
    echo "  --topic TOPIC           Kafka topic to consume from"
    echo ""
    echo "Optional:"
    echo "  --brokers HOST:PORT     Kafka brokers (default: localhost:9092)"
    echo "  --security PROTOCOL     Security protocol: PLAINTEXT|SSL|SASL_SSL (default: PLAINTEXT)"
    echo "  --ssl-port PORT         SSL port when using SSL/SASL_SSL (default: 9093)"
    echo "  --count N               Maximum number of messages to consume"
    echo "  --timeout SECONDS       Timeout in seconds (default: 30)"
    echo "  --follow                Keep consuming messages (like tail -f)"
    echo "  --compact               Show compact output without metadata"
    echo "  --help                  Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 --topic test-topic --security PLAINTEXT"
    echo "  $0 --topic ktranslate-ssl-ultra-debug --security SSL"
    echo "  $0 --topic ktranslate-comprehensive-debug --security SASL_SSL --follow"
    echo ""
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

check_dependencies() {
    # Check for kafkacat or kcat
    if command -v kcat &> /dev/null; then
        KAFKACAT_CMD="kcat"
        return 0
    elif command -v kafkacat &> /dev/null; then
        KAFKACAT_CMD="kafkacat"
        return 0
    else
        log_error "Neither 'kcat' nor 'kafkacat' found. Please install one of them:"
        echo ""
        echo "Ubuntu/Debian:"
        echo "  sudo apt-get update && sudo apt-get install kafkacat"
        echo ""
        echo "Or install kcat (newer version):"
        echo "  wget https://github.com/edenhill/kcat/releases/download/1.7.1/kcat-1.7.1-linux-musl-amd64.tar.gz"
        echo "  tar -xzf kcat-1.7.1-linux-musl-amd64.tar.gz"
        echo "  sudo mv kcat /usr/local/bin/"
        echo ""
        return 1
    fi
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --topic)
                TOPIC="$2"
                shift 2
                ;;
            --brokers)
                BROKERS="$2"
                shift 2
                ;;
            --security)
                SECURITY="$2"
                shift 2
                ;;
            --ssl-port)
                SSL_PORT="$2"
                shift 2
                ;;
            --count)
                COUNT="$2"
                shift 2
                ;;
            --timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            --follow)
                FOLLOW=true
                shift
                ;;
            --compact)
                COMPACT=true
                shift
                ;;
            --help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    if [[ -z "$TOPIC" ]]; then
        log_error "Topic is required"
        usage
        exit 1
    fi
}

build_kafkacat_args() {
    local args=()
    
    # Basic consumer configuration
    args+=("-C")  # Consumer mode
    args+=("-t" "$TOPIC")
    args+=("-b" "$BROKERS")
    
    # Security configuration
    case "$SECURITY" in
        "PLAINTEXT")
            # No additional args needed
            ;;
        "SSL")
            args+=("-X" "security.protocol=SSL")
            args+=("-X" "ssl.ca.location=")
            args+=("-X" "ssl.endpoint.identification.algorithm=none")
            args+=("-X" "enable.ssl.certificate.verification=false")
            ;;
        "SASL_SSL")
            args+=("-X" "security.protocol=SASL_SSL")
            args+=("-X" "sasl.mechanism=GSSAPI")
            args+=("-X" "ssl.ca.location=")
            args+=("-X" "ssl.endpoint.identification.algorithm=none")
            args+=("-X" "enable.ssl.certificate.verification=false")
            ;;
        *)
            log_error "Unsupported security protocol: $SECURITY"
            exit 1
            ;;
    esac
    
    # Consumer behavior
    args+=("-o" "beginning")  # Start from beginning (earliest messages)
    
    if [[ -n "$COUNT" ]]; then
        args+=("-c" "$COUNT")
    fi
    
    if [[ "$FOLLOW" == false ]]; then
        args+=("-e")  # Exit when last message in partition has been received
    fi
    
    # Output format
    if [[ "$COMPACT" == true ]]; then
        args+=("-f" "%s\n")  # Just the message value
    else
        args+=("-f" "Topic: %t, Partition: %p, Offset: %o, Timestamp: %T\nKey: %k\nValue: %s\n---\n")
    fi
    
    echo "${args[@]}"
}

main() {
    parse_args "$@"
    
    if ! check_dependencies; then
        exit 1
    fi
    
    # Adjust broker port for SSL/SASL_SSL
    if [[ "$SECURITY" == "SSL" || "$SECURITY" == "SASL_SSL" ]] && [[ "$BROKERS" == "localhost:9092" ]]; then
        BROKERS="localhost:${SSL_PORT}"
    fi
    
    log_info "Kafka Message Verifier starting..."
    log_info "Tool: $KAFKACAT_CMD"
    log_info "Topic: $TOPIC"
    log_info "Brokers: $BROKERS"
    log_info "Security: $SECURITY"
    log_info "Timeout: ${TIMEOUT}s"
    
    if [[ -n "$COUNT" ]]; then
        log_info "Max messages: $COUNT"
    fi
    
    if [[ "$FOLLOW" == true ]]; then
        log_info "Following mode: will keep consuming until interrupted"
    fi
    
    echo "$(printf '%.0s-' {1..50})"
    
    # Build kafkacat arguments
    local kafkacat_args
    read -ra kafkacat_args <<< "$(build_kafkacat_args)"
    
    log_info "Consuming from topic '$TOPIC'..."
    log_info "Waiting for messages... (Press Ctrl+C to stop)"
    echo ""
    
    # Set timeout if not following
    if [[ "$FOLLOW" == false ]]; then
        timeout "$TIMEOUT" "$KAFKACAT_CMD" "${kafkacat_args[@]}" 2>/dev/null || {
            local exit_code=$?
            if [[ $exit_code -eq 124 ]]; then
                echo ""
                log_warning "Timeout reached (${TIMEOUT}s), no messages received"
            elif [[ $exit_code -eq 1 ]]; then
                echo ""
                log_info "No messages in topic or reached end of topic"
            fi
        }
    else
        "$KAFKACAT_CMD" "${kafkacat_args[@]}"
    fi
    
    log_success "Message verification completed"
}

# Run main function with all arguments
main "$@"