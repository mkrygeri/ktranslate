# KTranslate Kafka + Kerberos Testing Environment

This directory contains a complete testing environment for validating KTranslate's new Kafka Kerberos authentication and cache-based rollup features.

## 🚀 Quick Start

```bash
cd testing

# Make scripts executable
chmod +x *.sh

# Set up the environment (takes ~3-5 minutes)
./setup.sh

# Run comprehensive tests
./test.sh

# Clean up when done
./cleanup.sh
```

## 🏗️ Environment Components

### Services
- **KDC (Kerberos)**: `kdc.example.com:88` - Authentication server
- **Zookeeper**: `zookeeper.example.com:2181` - Kafka coordination
- **Kafka Broker**: `kafka.example.com:9094` (SASL_SSL) / `localhost:9092` (PLAINTEXT)
- **Kafka UI**: `http://localhost:8080` - Web interface for monitoring

### Authentication Types
- **PLAINTEXT**: No authentication (development/testing)
- **SASL_SSL**: Kerberos GSSAPI with SSL encryption  
- **SSL**: SSL encryption only (mutual TLS optional)

## 📋 Test Scenarios

### 1. Basic Connectivity Tests
- PLAINTEXT connection validation
- SSL-only connection with certificate validation
- Kerberos GSSAPI authentication with SSL

### 2. Cache-Based Rollup Testing
- Memory limit validation (`rollup_max_memory_mb`)
- Key count limits (`rollup_max_keys`)
- Emergency aging when cache is full (`rollup_emergency_cleanup`)
- Performance comparison vs HLL

### 3. Security Validation
- Certificate validation and hostname verification
- Kerberos ticket generation and renewal
- SASL mechanism negotiation
- SSL/TLS protocol validation

## 🔧 Configuration Files Generated

```
testing/
├── docker-compose.yml          # Container orchestration
├── kerberos/
│   ├── krb5.conf              # Kerberos client configuration
│   ├── kafka_server_jaas.conf # Kafka server authentication
│   ├── zookeeper_jaas.conf    # Zookeeper authentication
│   └── *.keytab               # Service principals
├── ssl/
│   ├── ca-cert                # Certificate Authority
│   ├── kafka.server.keystore.jks
│   ├── kafka.server.truststore.jks
│   └── *_creds                # Credential files
└── logs/                      # Service logs
```

## 🧪 Manual Testing Commands

### Test Kerberos Authentication
```bash
# Check principal
docker exec krb5-kdc klist -kt /tmp/keytabs/ktranslate.keytab

# Test kinit
docker exec krb5-kdc kinit -kt /tmp/keytabs/ktranslate.keytab ktranslate@EXAMPLE.COM
```

### Test Kafka Topics
```bash
# List topics
docker exec kafka-broker kafka-topics --bootstrap-server localhost:9092 --list

# Create test topic
docker exec kafka-broker kafka-topics --bootstrap-server localhost:9092 \
    --create --topic test --partitions 3 --replication-factor 1

# Consume messages
docker exec kafka-broker kafka-console-consumer \
    --bootstrap-server localhost:9092 --topic ktranslate-metrics --from-beginning
```

### Test KTranslate with Custom Config
```bash
# Build ktranslate
cd .. && make && cd testing

# Run with Kerberos
../ktranslate \
    -kentik_api_device_name="test-device" \
    -kentik_api_plan_id=1 \
    -metrics=kafka \
    -kafka_brokers="kafka.example.com:9094" \
    -kafka_topic="ktranslate-metrics" \
    -kafka_security_protocol=SASL_SSL \
    -kafka_sasl_mechanism=GSSAPI \
    -kafka_kerberos_config_path=./kerberos/krb5.conf \
    -kafka_kerberos_service_name=kafka \
    -kafka_kerberos_keytab_path=./kerberos/ktranslate.keytab \
    -kafka_kerberos_principal=ktranslate@EXAMPLE.COM \
    -kafka_ssl_ca_location=./ssl/ca-cert \
    -rollup_max_memory_mb=256 \
    -rollup_max_keys=50000 \
    -rollup_emergency_cleanup=0.8 \
    -format=json
```

## 📊 Performance Testing

### Memory Usage Monitoring
```bash
# Monitor container memory
docker stats

# Monitor Java heap (Kafka)
docker exec kafka-broker jps -v
```

### Cache Performance Metrics
The test script generates synthetic flow data to validate:
- Cache memory estimation accuracy
- Emergency cleanup triggering at 80% threshold  
- Key count limits enforcement
- Aggregation performance vs HLL

### Expected Performance
- **Throughput**: 10,000+ flows/second with cache-based rollup
- **Memory**: ~256MB for 50,000 unique keys
- **Latency**: Sub-millisecond cache lookups
- **Emergency Cleanup**: Removes oldest 20% of entries when triggered

## 🛠️ Troubleshooting

### Common Issues

#### Kerberos Authentication Fails
```bash
# Check KDC is running
docker exec krb5-kdc systemctl status krb5-kdc

# Verify principal exists
docker exec krb5-kdc kadmin.local -q "listprincs"

# Test keytab
docker exec krb5-kdc klist -kt /tmp/keytabs/ktranslate.keytab
```

#### SSL Certificate Issues
```bash
# Verify certificate
openssl x509 -in ssl/ca-cert -text -noout

# Check keystore
keytool -list -keystore ssl/kafka.server.keystore.jks -storepass password
```

#### Kafka Connection Issues
```bash
# Check Kafka logs
docker-compose logs kafka

# Test basic connectivity
docker exec kafka-broker kafka-broker-api-versions --bootstrap-server localhost:9092
```

### Log Locations
- Kafka: `docker-compose logs kafka`
- Kerberos: `docker-compose logs kdc`
- KTranslate: Standard output
- System: `./logs/` directory

## 🔍 Validation Checklist

- [ ] All containers start successfully
- [ ] Kerberos principals created
- [ ] SSL certificates generated and valid
- [ ] PLAINTEXT Kafka connection works
- [ ] SASL_SSL Kafka connection works
- [ ] Topics created and messages sent
- [ ] Cache-based rollup functioning
- [ ] Memory limits respected
- [ ] Emergency cleanup triggered
- [ ] Performance meets expectations

## 🚦 Production Readiness

After successful testing, migrate to production by:
1. Using production Kerberos realm and KDC
2. Generating production SSL certificates
3. Configuring proper network security
4. Setting up monitoring and alerting
5. Implementing backup and disaster recovery
6. Tuning performance parameters based on load

## 📖 Related Documentation

- [Kafka Security Documentation](https://kafka.apache.org/documentation/#security)
- [Kerberos Authentication Guide](https://web.mit.edu/kerberos/krb5-latest/doc/)
- [Sarama Library Documentation](https://pkg.go.dev/github.com/Shopify/sarama)
- [KTranslate Configuration Reference](../README.md)
