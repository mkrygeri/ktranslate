# Testing Kafka Kerberos Implementation - Real Environment Guide

## Overview

This guide provides several approaches to test the new Kafka Kerberos implementation in ktranslate, ranging from simple local testing to full production-like environments.

## Testing Approaches

### 1. 🐳 **Docker-based Testing (Recommended for Quick Testing)**

This is the fastest way to get a complete Kafka + Kerberos environment running locally.

#### Create Docker Compose Setup

```yaml
# docker-compose-kafka-kerberos.yml
version: '3.8'
services:
  kdc:
    image: gcavalcante8808/krb5-server
    hostname: kdc.example.com
    environment:
      - REALM=EXAMPLE.COM
      - SUPPORTED_ENCRYPTION_TYPES=aes256-cts-hmac-sha1-96:normal
      - KADMIN_PRINCIPAL=admin/admin
      - KADMIN_PASSWORD=admin
    volumes:
      - ./kerberos:/tmp/keytabs
    ports:
      - "88:88"
      - "464:464"
      - "749:749"
    networks:
      - kafka-net

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    hostname: zookeeper.example.com
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
      KAFKA_OPTS: "-Djava.security.auth.login.config=/etc/kafka/zookeeper_jaas.conf
                   -Djava.security.krb5.conf=/etc/kafka/krb5.conf"
    volumes:
      - ./kerberos/zookeeper_jaas.conf:/etc/kafka/zookeeper_jaas.conf
      - ./kerberos/krb5.conf:/etc/kafka/krb5.conf
      - ./kerberos:/tmp/keytabs
    depends_on:
      - kdc
    networks:
      - kafka-net

  kafka:
    image: confluentinc/cp-kafka:latest
    hostname: kafka.example.com
    depends_on:
      - zookeeper
      - kdc
    ports:
      - "9094:9094"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper.example.com:2181
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: SASL_SSL:SASL_SSL,PLAINTEXT:PLAINTEXT
      KAFKA_ADVERTISED_LISTENERS: SASL_SSL://kafka.example.com:9094,PLAINTEXT://localhost:9092
      KAFKA_SASL_MECHANISM_INTER_BROKER_PROTOCOL: GSSAPI
      KAFKA_SASL_ENABLED_MECHANISMS: GSSAPI
      KAFKA_SASL_KERBEROS_SERVICE_NAME: kafka
      KAFKA_SSL_KEYSTORE_FILENAME: kafka.server.keystore.jks
      KAFKA_SSL_KEYSTORE_CREDENTIALS: kafka_keystore_creds
      KAFKA_SSL_KEY_CREDENTIALS: kafka_ssl_key_creds
      KAFKA_SSL_TRUSTSTORE_FILENAME: kafka.server.truststore.jks
      KAFKA_SSL_TRUSTSTORE_CREDENTIALS: kafka_truststore_creds
      KAFKA_SSL_CLIENT_AUTH: required
      KAFKA_OPTS: "-Djava.security.auth.login.config=/etc/kafka/kafka_server_jaas.conf
                   -Djava.security.krb5.conf=/etc/kafka/krb5.conf"
    volumes:
      - ./kerberos/kafka_server_jaas.conf:/etc/kafka/kafka_server_jaas.conf
      - ./kerberos/krb5.conf:/etc/kafka/krb5.conf
      - ./kerberos:/tmp/keytabs
      - ./ssl:/etc/kafka/secrets
    networks:
      - kafka-net

networks:
  kafka-net:
    driver: bridge
```

#### Setup Script

```bash
#!/bin/bash
# setup-test-env.sh

set -e

echo "Setting up Kafka Kerberos test environment..."

# Create directories
mkdir -p kerberos ssl

# Generate SSL certificates
./generate-ssl-certs.sh

# Generate Kerberos configuration
cat > kerberos/krb5.conf << EOF
[libdefaults]
    default_realm = EXAMPLE.COM
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ticket_lifetime = 24h
    renew_lifetime = 7d
    forwardable = true

[realms]
    EXAMPLE.COM = {
        kdc = kdc.example.com:88
        admin_server = kdc.example.com:749
        default_domain = example.com
    }

[domain_realm]
    .example.com = EXAMPLE.COM
    example.com = EXAMPLE.COM
EOF

# Kafka server JAAS configuration
cat > kerberos/kafka_server_jaas.conf << EOF
KafkaServer {
    com.sun.security.auth.module.Krb5LoginModule required
    useKeyTab=true
    storeKey=true
    keyTab="/tmp/keytabs/kafka.keytab"
    principal="kafka/kafka.example.com@EXAMPLE.COM";
};
EOF

# Zookeeper JAAS configuration
cat > kerberos/zookeeper_jaas.conf << EOF
Server {
    com.sun.security.auth.module.Krb5LoginModule required
    useKeyTab=true
    storeKey=true
    keyTab="/tmp/keytabs/zookeeper.keytab"
    principal="zookeeper/zookeeper.example.com@EXAMPLE.COM";
};
EOF

echo "Starting services..."
docker-compose -f docker-compose-kafka-kerberos.yml up -d

echo "Waiting for services to start..."
sleep 30

# Create Kerberos principals and keytabs
echo "Creating Kerberos principals..."
docker exec -it $(docker-compose -f docker-compose-kafka-kerberos.yml ps -q kdc) bash -c "
    kadmin.local -q 'addprinc -pw kafka kafka/kafka.example.com@EXAMPLE.COM'
    kadmin.local -q 'addprinc -pw zookeeper zookeeper/zookeeper.example.com@EXAMPLE.COM'
    kadmin.local -q 'addprinc -pw ktranslate ktranslate@EXAMPLE.COM'
    kadmin.local -q 'ktadd -k /tmp/keytabs/kafka.keytab kafka/kafka.example.com@EXAMPLE.COM'
    kadmin.local -q 'ktadd -k /tmp/keytabs/zookeeper.keytab zookeeper/zookeeper.example.com@EXAMPLE.COM'
    kadmin.local -q 'ktadd -k /tmp/keytabs/ktranslate.keytab ktranslate@EXAMPLE.COM'
"

# Fix keytab permissions
chmod 644 kerberos/*.keytab

echo "Environment ready! Test with:"
echo "./ktranslate -sinks kafka -kafka_topic test -bootstrap.servers kafka.example.com:9094 -kafka_security_protocol SASL_SSL -kafka_sasl_mechanism GSSAPI -kafka_kerberos_principal ktranslate@EXAMPLE.COM -kafka_kerberos_keytab_path ./kerberos/ktranslate.keytab"
```

#### SSL Certificate Generation Script

```bash
#!/bin/bash
# generate-ssl-certs.sh

mkdir -p ssl

# Generate CA
openssl req -new -x509 -keyout ssl/ca-key -out ssl/ca-cert -days 365 -nodes \
    -subj "/C=US/ST=CA/L=San Francisco/O=Example/CN=ca.example.com"

# Generate server key and certificate
openssl req -new -keyout ssl/kafka.server.key -out ssl/kafka.server.csr -nodes \
    -subj "/C=US/ST=CA/L=San Francisco/O=Example/CN=kafka.example.com"

openssl x509 -req -CA ssl/ca-cert -CAkey ssl/ca-key -in ssl/kafka.server.csr \
    -out ssl/kafka.server.crt -days 365 -CAcreateserial

# Convert to JKS format for Kafka
keytool -keystore ssl/kafka.server.keystore.jks -alias localhost -validity 365 \
    -genkey -keyalg RSA -storepass password -keypass password \
    -dname "CN=kafka.example.com, OU=Example, O=Example, L=San Francisco, ST=CA, C=US"

keytool -keystore ssl/kafka.server.truststore.jks -alias CARoot -import -file ssl/ca-cert \
    -storepass password -noprompt

# Create password files
echo "password" > ssl/kafka_keystore_creds
echo "password" > ssl/kafka_ssl_key_creds  
echo "password" > ssl/kafka_truststore_creds

chmod 644 ssl/*
```

### 2. 🏢 **Testing Against Existing Kerberos Infrastructure**

If you have access to an existing Kerberos-enabled Kafka cluster:

#### Test Configuration

```bash
#!/bin/bash
# test-existing-kafka.sh

# Set your environment variables
export KAFKA_BROKERS="your-kafka-broker1:9094,your-kafka-broker2:9094"
export KAFKA_TOPIC="test-ktranslate"
export KERBEROS_PRINCIPAL="your-service-account@YOUR-REALM.COM"
export KEYTAB_PATH="/path/to/your.keytab"
export KRB5_CONFIG="/etc/krb5.conf"

# Test basic connectivity first
echo "Testing Kafka connectivity..."
./ktranslate \
  -sinks kafka \
  -kafka_topic $KAFKA_TOPIC \
  -bootstrap.servers $KAFKA_BROKERS \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_principal $KERBEROS_PRINCIPAL \
  -kafka_kerberos_keytab_path $KEYTAB_PATH \
  -kafka_kerberos_config_path $KRB5_CONFIG \
  -log_level debug \
  -format json \
  -nf.source auto \
  -nf.addr 0.0.0.0 \
  -nf.port 9995

# To test with actual flow data, send some test flows:
# nfcapd or softflowd can generate test traffic
```

### 3. 🧪 **Unit Testing with Mock Kafka**

Create a comprehensive test that validates the configuration without needing actual Kafka:

```go
// test-kafka-config.go
package main

import (
    "context"
    "fmt"
    "log"

    go_metrics "github.com/kentik/go-metrics"
    "github.com/kentik/ktranslate"
    "github.com/kentik/ktranslate/pkg/eggs/logger"
    lt "github.com/kentik/ktranslate/pkg/eggs/logger/testing"
    "github.com/kentik/ktranslate/pkg/formats"
    "github.com/kentik/ktranslate/pkg/kt"
    "github.com/kentik/ktranslate/pkg/sinks/kafka"
)

func main() {
    // Test various Kerberos configurations
    testConfigs := []ktranslate.KafkaSinkConfig{
        {
            Topic:                   "test-topic",
            BootstrapServers:        "kafka1:9094,kafka2:9094",
            SecurityProtocol:        "SASL_SSL",
            SASLMechanism:          "GSSAPI",
            KerberosServiceName:    "kafka",
            KerberosRealm:          "EXAMPLE.COM",
            KerberosPrincipal:      "ktranslate@EXAMPLE.COM",
            KerberosKeytabPath:     "/etc/keytabs/ktranslate.keytab",
            KerberosConfigPath:     "/etc/krb5.conf",
            RequiredAcks:           -1,
            Compression:            "snappy",
        },
        {
            Topic:            "test-topic-ssl",
            BootstrapServers: "kafka-ssl:9093",
            SecurityProtocol: "SSL",
            SSLCAFile:        "/etc/ssl/ca-cert.pem",
            SSLCertFile:      "/etc/ssl/client-cert.pem",
            SSLKeyFile:       "/etc/ssl/client-key.pem",
        },
        {
            Topic:            "test-topic-plaintext",
            BootstrapServers: "kafka-plain:9092",
            SecurityProtocol: "PLAINTEXT",
        },
    }

    registry := go_metrics.NewRegistry()
    
    for i, cfg := range testConfigs {
        fmt.Printf("\n=== Testing Configuration %d ===\n", i+1)
        fmt.Printf("Security Protocol: %s\n", cfg.SecurityProtocol)
        fmt.Printf("SASL Mechanism: %s\n", cfg.SASLMechanism)
        fmt.Printf("Bootstrap Servers: %s\n", cfg.BootstrapServers)
        
        // Create logger for this test
        testLogger := lt.NewTestLogger()
        log := testLogger.GetUnderlyingLogger()
        
        // Create sink
        sink, err := kafka.NewSink(log, registry, &cfg)
        if err != nil {
            fmt.Printf("❌ Failed to create sink: %v\n", err)
            continue
        }
        
        // Test initialization (this validates all the configuration)
        ctx := context.Background()
        err = sink.Init(ctx, formats.FORMAT_JSON, kt.CompressionNone, nil)
        if err != nil {
            fmt.Printf("❌ Failed to initialize sink: %v\n", err)
            // This is expected for invalid configs, continue
        } else {
            fmt.Printf("✅ Sink initialized successfully\n")
            
            // Test sending a message (will fail without real Kafka, but validates message creation)
            testPayload := kt.NewOutput([]byte(`{"test": "message"}`))
            sink.Send(ctx, testPayload)
            fmt.Printf("✅ Message send attempted\n")
            
            // Clean up
            sink.Close()
            fmt.Printf("✅ Sink closed cleanly\n")
        }
    }
}
```

### 4. 📊 **Performance Testing**

Once basic connectivity is working, test performance and reliability:

```bash
#!/bin/bash
# performance-test.sh

export KAFKA_BROKERS="kafka1:9094,kafka2:9094"
export KAFKA_TOPIC="performance-test"

echo "Starting performance test..."

# Test with high message throughput
./ktranslate \
  -sinks kafka \
  -kafka_topic $KAFKA_TOPIC \
  -bootstrap.servers $KAFKA_BROKERS \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_principal ktranslate@EXAMPLE.COM \
  -kafka_kerberos_keytab_path /etc/keytabs/ktranslate.keytab \
  -kafka_compression snappy \
  -kafka_flush_messages 1000 \
  -kafka_flush_bytes 1048576 \
  -kafka_flush_frequency 100 \
  -kafka_required_acks 1 \
  -max_flows_per_message 10000 \
  -threads 4 \
  -log_level info \
  -format json &

KTRANSLATE_PID=$!

# Generate test traffic (use nfcapd, softflowd, or similar)
echo "Generating test traffic..."
# Insert your traffic generation method here

# Monitor for 5 minutes
sleep 300

# Stop ktranslate
kill $KTRANSLATE_PID

echo "Performance test completed. Check Kafka topic for messages."
```

### 5. 🔍 **Troubleshooting Tools**

#### Debug Configuration Script

```bash
#!/bin/bash
# debug-kafka-kerberos.sh

echo "=== Kafka Kerberos Debug Information ==="

echo "1. Checking Kerberos configuration..."
echo "KRB5_CONFIG: $KRB5_CONFIG"
if [ -f "$KRB5_CONFIG" ]; then
    echo "✅ krb5.conf exists"
    echo "Realms configured:"
    grep -A 5 "^\[realms\]" $KRB5_CONFIG || echo "❌ No realms section found"
else
    echo "❌ krb5.conf not found at $KRB5_CONFIG"
fi

echo -e "\n2. Checking keytab..."
KEYTAB_PATH=${KEYTAB_PATH:-"/etc/keytabs/ktranslate.keytab"}
if [ -f "$KEYTAB_PATH" ]; then
    echo "✅ Keytab exists at $KEYTAB_PATH"
    echo "Principals in keytab:"
    klist -kt $KEYTAB_PATH 2>/dev/null || echo "❌ Cannot read keytab (permission issue?)"
else
    echo "❌ Keytab not found at $KEYTAB_PATH"
fi

echo -e "\n3. Testing Kerberos authentication..."
kinit -kt $KEYTAB_PATH ktranslate@EXAMPLE.COM 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ Kerberos authentication successful"
    klist | head -5
else
    echo "❌ Kerberos authentication failed"
fi

echo -e "\n4. Testing Kafka connectivity..."
KAFKA_BROKERS=${KAFKA_BROKERS:-"localhost:9094"}
timeout 10 bash -c "cat < /dev/null > /dev/tcp/${KAFKA_BROKERS%%:*}/${KAFKA_BROKERS##*:}" 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ Kafka broker is reachable"
else
    echo "❌ Cannot connect to Kafka broker at $KAFKA_BROKERS"
fi

echo -e "\n5. Running ktranslate with debug logging..."
./ktranslate \
  -sinks kafka \
  -kafka_topic debug-test \
  -bootstrap.servers $KAFKA_BROKERS \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_principal ktranslate@EXAMPLE.COM \
  -kafka_kerberos_keytab_path $KEYTAB_PATH \
  -log_level debug \
  -max_flows_per_message 1 \
  2>&1 | head -50

echo -e "\n=== Debug completed ==="
```

### 6. 🎯 **Validation Checklist**

Before deploying to production, verify:

#### ✅ **Configuration Validation**
- [ ] Kerberos principal can authenticate successfully
- [ ] Keytab file has correct permissions (readable by ktranslate process)
- [ ] krb5.conf points to correct KDC
- [ ] Kafka brokers are accessible on specified ports
- [ ] SSL certificates are valid (if using SSL)

#### ✅ **Functional Testing** 
- [ ] ktranslate can connect to Kafka successfully
- [ ] Messages are being produced to the correct topic
- [ ] No authentication errors in logs
- [ ] Performance is acceptable under load
- [ ] Failover works if multiple brokers configured

#### ✅ **Security Validation**
- [ ] Traffic is encrypted (if using SSL/SASL_SSL)
- [ ] Authentication logs show successful Kerberos auth
- [ ] No credentials are logged or exposed
- [ ] Keytab renewal works (if applicable)

#### ✅ **Monitoring Setup**
- [ ] Kafka producer metrics are being collected
- [ ] Error rates are monitored
- [ ] Kerberos ticket expiry is monitored
- [ ] Kafka topic lag is monitored

## Quick Start Test Command

For immediate testing with minimal setup:

```bash
# Build ktranslate
go build ./cmd/ktranslate

# Test configuration validation (will fail to connect but validate config)
./ktranslate \
  -sinks kafka \
  -kafka_topic test \
  -bootstrap.servers localhost:9094 \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_principal test@EXAMPLE.COM \
  -kafka_kerberos_keytab_path /tmp/test.keytab \
  -log_level debug \
  -generate-config > test-config.yaml

# Check the generated config contains all Kafka parameters
grep -A 30 "kafkasink:" test-config.yaml
```

This comprehensive testing approach will help you validate the Kerberos implementation in various scenarios, from local development to production deployment! 🚀
