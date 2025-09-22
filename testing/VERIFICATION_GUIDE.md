# Kafka Message Verification Guide

This directory contains tools to verify that ktranslate is successfully sending messages to Kafka brokers with different security configurations.

## Available Verification Tools

### 1. Simple Bash Script with kcat (`kafka-verifier.sh`)
**Recommended** - Uses the system kcat tool, no additional dependencies needed.

### 2. Python Script (`kafka-message-verifier.py`)
Advanced Python script with more features, requires `kafka-python` library.

### 3. Setup Scripts
- `setup-verifier.sh` - Installs Python dependencies for the Python verifier
- `test-sasl-ssl-debug.sh` - Comprehensive infrastructure testing

## Quick Start Examples

### Testing PLAINTEXT Connection
```bash
# Start a simple Kafka broker first
cd /home/mikek/kafka-sasl-ssl-lab
docker run --rm -d --name kafka-simple-test \
  -p 9092:9092 \
  -e KAFKA_BROKER_ID=1 \
  -e KAFKA_LISTENERS=PLAINTEXT://0.0.0.0:9092 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
  -e KAFKA_ZOOKEEPER_CONNECT=zookeeper-fresh:2181 \
  --link zookeeper-fresh:zookeeper \
  confluentinc/cp-kafka:7.4.0

# Test with ktranslate PLAINTEXT config
cd /home/mikek/ktranslate
./ktranslate -config=testing/ktranslate-plaintext.yaml -log_level=debug -stdout -nf.source=netflow5 -nf.port=9995 &

# Verify messages in another terminal
cd /home/mikek/ktranslate/testing
./kafka-verifier.sh --topic ktranslate-plaintext --security PLAINTEXT --follow
```

### Testing SSL Connection
```bash
# Start ktranslate with SSL config
cd /home/mikek/ktranslate
./ktranslate -config=testing/ktranslate-ssl-ultra-debug.yaml -log_level=debug -stdout -nf.source=netflow5 -nf.port=9995 &

# Verify messages (will auto-detect SSL port 9093)
cd /home/mikek/ktranslate/testing
./kafka-verifier.sh --topic ktranslate-ssl-ultra-debug --security SSL --follow
```

### Testing SASL_SSL Connection
```bash
# Ensure KDC and SASL_SSL broker are running
cd /home/mikek/kafka-sasl-ssl-lab
docker ps | grep -E "(ktranslate-kdc|kafka.*sasl)"

# Start ktranslate with SASL_SSL config
cd /home/mikek/ktranslate
export KRB5_TRACE=/dev/stderr
export GODEBUG=tls=1
./ktranslate -config=testing/ktranslate-comprehensive-debug.yaml -log_level=debug -stdout -nf.source=netflow5 -nf.port=9995 &

# Verify messages
cd /home/mikek/ktranslate/testing
./kafka-verifier.sh --topic ktranslate-comprehensive-debug --security SASL_SSL --follow
```

## Detailed Usage

### kafka-verifier.sh Options
```bash
./kafka-verifier.sh --help

Required:
  --topic TOPIC           Kafka topic to consume from

Optional:
  --brokers HOST:PORT     Kafka brokers (default: localhost:9092)
  --security PROTOCOL     Security protocol: PLAINTEXT|SSL|SASL_SSL (default: PLAINTEXT)
  --ssl-port PORT         SSL port when using SSL/SASL_SSL (default: 9093)
  --count N               Maximum number of messages to consume
  --timeout SECONDS       Timeout in seconds (default: 30)
  --follow                Keep consuming messages (like tail -f)
  --compact               Show compact output without metadata
```

### Examples

#### 1. Check for any messages (with timeout)
```bash
./kafka-verifier.sh --topic ktranslate-ssl-ultra-debug --security SSL --timeout 10
```

#### 2. Follow messages continuously
```bash
./kafka-verifier.sh --topic ktranslate-ssl-ultra-debug --security SSL --follow
```

#### 3. Get first 5 messages only
```bash
./kafka-verifier.sh --topic ktranslate-ssl-ultra-debug --security SSL --count 5
```

#### 4. Compact output (just message content)
```bash
./kafka-verifier.sh --topic ktranslate-ssl-ultra-debug --security SSL --compact --count 3
```

#### 5. Custom broker and port
```bash
./kafka-verifier.sh --topic test --brokers kafka.example.com:9093 --security SSL
```

## Testing Workflow

### 1. Infrastructure Check
```bash
# Check running containers
docker ps

# Should see:
# - zookeeper container
# - kafka container (with appropriate security)
# - ktranslate-kdc (for SASL_SSL)
```

### 2. Start ktranslate
```bash
cd /home/mikek/ktranslate

# Choose appropriate config based on your Kafka setup:
# - ktranslate-ssl-ultra-debug.yaml for SSL
# - ktranslate-comprehensive-debug.yaml for SASL_SSL
./ktranslate -config=testing/YOUR_CONFIG.yaml -log_level=debug -stdout -nf.source=netflow5 -nf.port=9995 &
```

### 3. Generate Test Traffic
```bash
# Send test NetFlow data to ktranslate
echo "Test flow data" | nc -u localhost 9995

# Or use nfcapd if available
# nfcapd -T all -l . -p 9995
```

### 4. Verify Messages
```bash
cd /home/mikek/ktranslate/testing

# Match security protocol to your ktranslate config
./kafka-verifier.sh --topic YOUR_TOPIC --security YOUR_SECURITY --follow
```

### 5. Troubleshooting
```bash
# Check ktranslate logs
tail -f /tmp/ktranslate.log

# Check Kafka broker logs
docker logs KAFKA_CONTAINER_NAME

# Test Kafka connectivity directly
kcat -b localhost:9093 -L -X security.protocol=SSL
```

## Message Format

ktranslate typically sends JSON messages with flow data. Expected message structure:
```json
{
  "timestamp": 1694975994,
  "src_addr": "192.168.1.1",
  "dst_addr": "192.168.1.2",
  "src_port": 80,
  "dst_port": 443,
  "protocol": 6,
  "in_bytes": 1500,
  "out_bytes": 800,
  "in_pkts": 1,
  "out_pkts": 1,
  "device_name": "router1"
}
```

## Common Issues

### 1. "No messages received"
- Check if ktranslate is running: `ps aux | grep ktranslate`
- Verify topic name matches config file
- Check if Kafka broker is accessible: `kcat -b localhost:9092 -L`

### 2. "SSL/SASL connection failed"
- Verify certificates are in place
- Check Kerberos authentication (for SASL_SSL)
- Review ktranslate debug logs

### 3. "Permission denied" or "Topic not found"
- Kafka may auto-create topics, wait a few seconds
- Check Kafka broker logs for errors
- Verify security credentials

## Performance Testing

### High Volume Testing
```bash
# Follow messages with timestamps to verify throughput
./kafka-verifier.sh --topic YOUR_TOPIC --security SSL --follow | while read line; do
  echo "$(date): $line"
done
```

### Message Count Verification
```bash
# Count messages over specific time period
timeout 60 ./kafka-verifier.sh --topic YOUR_TOPIC --security SSL --follow --compact | wc -l
```

This provides comprehensive verification that your ktranslate SSL/SASL configuration is working correctly!