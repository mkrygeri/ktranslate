# Kafka Sink with Kerberos Authentication

## Overview

The Kafka sink in ktranslate has been updated to use the Shopify Sarama library, which provides full support for Kerberos (GSSAPI) authentication and other advanced security protocols. This replaces the previous segmentio/kafka-go implementation that did not support Kerberos.

## Security Protocols Supported

### 1. PLAINTEXT (Default)
Basic unencrypted connection - suitable for development or trusted networks.

```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -bootstrap.servers broker1:9092,broker2:9092 \
  -kafka_security_protocol PLAINTEXT
```

### 2. SASL_PLAINTEXT
SASL authentication over unencrypted connection.

```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -bootstrap.servers broker1:9092,broker2:9092 \
  -kafka_security_protocol SASL_PLAINTEXT \
  -kafka_sasl_mechanism PLAIN \
  -kafka_sasl_username myuser \
  -kafka_sasl_password mypassword
```

### 3. SSL
Encrypted connection with SSL/TLS.

```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -bootstrap.servers broker1:9093,broker2:9093 \
  -kafka_security_protocol SSL \
  -kafka_ssl_ca_file /path/to/ca-cert.pem \
  -kafka_ssl_cert_file /path/to/client-cert.pem \
  -kafka_ssl_key_file /path/to/client-key.pem
```

### 4. SASL_SSL
SASL authentication over encrypted SSL/TLS connection - most secure option.

```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -bootstrap.servers broker1:9094,broker2:9094 \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_ssl_ca_file /path/to/ca-cert.pem \
  -kafka_kerberos_service_name kafka \
  -kafka_kerberos_principal user@REALM.COM \
  -kafka_kerberos_realm REALM.COM \
  -kafka_kerberos_keytab_path /etc/security/keytabs/user.keytab
```

## Kerberos (GSSAPI) Authentication

### Prerequisites
1. **Kerberos client** installed (krb5-user on Ubuntu/Debian)
2. **krb5.conf** properly configured
3. **Valid keytab file** or **kinit** authentication
4. **Network connectivity** to Kerberos KDC

### Configuration Options

#### Basic Kerberos Setup
```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic secure-topic \
  -bootstrap.servers broker1:9094,broker2:9094 \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_service_name kafka \
  -kafka_kerberos_principal ktranslate@EXAMPLE.COM \
  -kafka_kerberos_realm EXAMPLE.COM
```

#### Keytab-based Authentication (Recommended for Production)
```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic secure-topic \
  -bootstrap.servers broker1:9094,broker2:9094 \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_service_name kafka \
  -kafka_kerberos_principal ktranslate@EXAMPLE.COM \
  -kafka_kerberos_realm EXAMPLE.COM \
  -kafka_kerberos_keytab_path /etc/security/keytabs/ktranslate.keytab \
  -kafka_kerberos_config_path /etc/krb5.conf
```

#### User-based Authentication (Requires kinit)
```bash
# First authenticate with Kerberos
kinit ktranslate@EXAMPLE.COM

# Then run ktranslate
./ktranslate \
  -sinks kafka \
  -kafka_topic secure-topic \
  -bootstrap.servers broker1:9094,broker2:9094 \
  -kafka_security_protocol SASL_SSL \
  -kafka_sasl_mechanism GSSAPI \
  -kafka_kerberos_service_name kafka \
  -kafka_kerberos_principal ktranslate@EXAMPLE.COM \
  -kafka_kerberos_realm EXAMPLE.COM
```

### Troubleshooting Kerberos

#### Common Issues and Solutions

1. **KDC unreachable**
   - Verify network connectivity to KDC
   - Check DNS resolution of KDC hostnames
   - Validate krb5.conf configuration

2. **Clock skew**
   - Ensure system time is synchronized (use NTP)
   - Maximum allowed skew is typically 5 minutes

3. **Principal not found**
   - Verify principal exists in KDC
   - Check principal name formatting (case sensitive)

4. **Keytab issues**
   - Verify keytab file permissions (readable by ktranslate user)
   - Check keytab contains correct principal: `klist -k /path/to/keytab`
   - Verify keytab encryption types match KDC configuration

5. **Service principal issues**
   - Ensure Kafka service principal exists: `kafka/broker-fqdn@REALM.COM`
   - Verify service name matches Kafka configuration

#### Debug Kerberos Authentication
```bash
# Enable Kerberos debugging
export KRB5_TRACE=/dev/stderr

# Test kinit
kinit -V ktranslate@EXAMPLE.COM

# Check current tickets
klist -v

# Test connectivity
kvno kafka/broker1.example.com@EXAMPLE.COM
```

## SASL Mechanisms

### PLAIN
Simple username/password authentication:
```bash
-kafka_sasl_mechanism PLAIN \
-kafka_sasl_username myuser \
-kafka_sasl_password mypassword
```

### SCRAM-SHA-256
Salted Challenge Response Authentication Mechanism:
```bash
-kafka_sasl_mechanism SCRAM-SHA-256 \
-kafka_sasl_username myuser \
-kafka_sasl_password mypassword
```

### SCRAM-SHA-512
More secure version of SCRAM:
```bash
-kafka_sasl_mechanism SCRAM-SHA-512 \
-kafka_sasl_username myuser \
-kafka_sasl_password mypassword
```

### GSSAPI (Kerberos)
See Kerberos section above.

## Producer Configuration

### Performance Tuning
```bash
# High throughput settings
./ktranslate \
  -sinks kafka \
  -kafka_topic high-volume-topic \
  -kafka_compression snappy \
  -kafka_flush_messages 1000 \
  -kafka_flush_bytes 1048576 \
  -kafka_flush_frequency 500 \
  -kafka_max_message_bytes 10485760

# Low latency settings  
./ktranslate \
  -sinks kafka \
  -kafka_topic low-latency-topic \
  -kafka_compression none \
  -kafka_flush_messages 1 \
  -kafka_flush_frequency 10 \
  -kafka_required_acks 1

# High reliability settings
./ktranslate \
  -sinks kafka \
  -kafka_topic reliable-topic \
  -kafka_required_acks -1 \
  -kafka_retry_max 10 \
  -kafka_compression gzip
```

### Compression Options
- `none`: No compression (fastest)
- `gzip`: Good compression ratio, moderate CPU usage
- `snappy`: Fast compression, lower ratio
- `lz4`: Very fast compression  
- `zstd`: Best compression ratio, higher CPU usage

### Acknowledgement Levels
- `0`: No acknowledgement (fire and forget)
- `1`: Leader acknowledgement only (default)
- `-1`: Full ISR acknowledgement (most reliable)

## Configuration File Example

Create a YAML configuration file for complex setups:

```yaml
# ktranslate-kafka.yaml
sinks:
  - kafka

kafkaSink:
  topic: "ktranslate-flows"
  bootstrapServers: "broker1:9094,broker2:9094,broker3:9094"
  securityProtocol: "SASL_SSL"
  saslMechanism: "GSSAPI"
  kerberosServiceName: "kafka"
  kerberosRealm: "EXAMPLE.COM"
  kerberosPrincipal: "ktranslate@EXAMPLE.COM"
  kerberosKeytabPath: "/etc/security/keytabs/ktranslate.keytab"
  kerberosConfigPath: "/etc/krb5.conf"
  sslCAFile: "/etc/ssl/certs/ca-certificates.crt"
  requiredAcks: -1
  compression: "snappy"
  maxMessageBytes: 10485760
  retryMax: 5
  flushFrequency: 100
  flushMessages: 100
  flushBytes: 65536
```

Then run:
```bash
./ktranslate -config ktranslate-kafka.yaml
```

## Migration from segmentio/kafka-go

The new Sarama-based implementation maintains backward compatibility for basic configurations:

### Old Configuration
```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -bootstrap.servers broker1:9092,broker2:9092
```

### New Configuration (Same Result)
```bash
./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -bootstrap.servers broker1:9092,broker2:9092 \
  -kafka_security_protocol PLAINTEXT
```

The key advantages of the new implementation:
- **Full Kerberos support** via GSSAPI mechanism
- **Advanced SSL/TLS configuration** with client certificates
- **Multiple SASL mechanisms** (PLAIN, SCRAM, GSSAPI)
- **Better error handling** and retry logic
- **Performance tuning options** for throughput and latency
- **Production-ready** security features

## Environment Variables

You can also set sensitive values via environment variables:

```bash
export KAFKA_SASL_PASSWORD="mypassword"
export KAFKA_SSL_KEY_PASSWORD="keypassword"

./ktranslate \
  -sinks kafka \
  -kafka_topic my-topic \
  -kafka_sasl_username myuser \
  # Password will be read from KAFKA_SASL_PASSWORD
```
