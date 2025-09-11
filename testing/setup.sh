#!/bin/bash

set -e

echo "🚀 Setting up Kafka + Kerberos Testing Environment"
echo "=================================================="

# Create necessary directories
mkdir -p kerberos ssl logs

# Generate Kerberos configuration
echo "📋 Creating Kerberos configuration files..."

cat > kerberos/krb5.conf << 'EOF'
[logging]
 default = FILE:/var/log/krb5libs.log
 kdc = FILE:/var/log/krb5kdc.log
 admin_server = FILE:/var/log/kadmind.log

[libdefaults]
 default_realm = EXAMPLE.COM
 dns_lookup_realm = false
 dns_lookup_kdc = false
 ticket_lifetime = 24h
 renew_lifetime = 7d
 forwardable = true
 default_tgs_enctypes = aes256-cts-hmac-sha1-96
 default_tkt_enctypes = aes256-cts-hmac-sha1-96

[realms]
 EXAMPLE.COM = {
  kdc = kdc.example.com:88
  admin_server = kdc.example.com:749
 }

[domain_realm]
 .example.com = EXAMPLE.COM
 example.com = EXAMPLE.COM
EOF

# Kafka server JAAS configuration
cat > kerberos/kafka_server_jaas.conf << 'EOF'
KafkaServer {
  com.sun.security.auth.module.Krb5LoginModule required
  useKeyTab=true
  storeKey=true
  keyTab="/var/keytabs/kafka.keytab"
  principal="kafka/kafka.example.com@EXAMPLE.COM";
};

Client {
  com.sun.security.auth.module.Krb5LoginModule required
  useKeyTab=true
  storeKey=true
  keyTab="/var/keytabs/kafka.keytab"
  principal="kafka/kafka.example.com@EXAMPLE.COM";
};
EOF

# Zookeeper JAAS configuration
cat > kerberos/zookeeper_jaas.conf << 'EOF'
Server {
  com.sun.security.auth.module.Krb5LoginModule required
  useKeyTab=true
  storeKey=true
  keyTab="/var/keytabs/zookeeper.keytab"
  principal="zookeeper/zookeeper.example.com@EXAMPLE.COM";
};
EOF

echo "🔐 Generating SSL certificates..."

# Generate SSL certificates for Kafka
cd ssl

# Create CA
openssl req -new -x509 -keyout ca-key -out ca-cert -days 365 -subj "/CN=ca.example.com" -nodes

# Create Kafka keystore
keytool -keystore kafka.server.keystore.jks -alias kafka -validity 365 -genkey -keyalg RSA -storepass password -keypass password -dname "CN=kafka.example.com,OU=Test,O=Test,L=Test,S=Test,C=US"

# Create certificate request
keytool -keystore kafka.server.keystore.jks -alias kafka -certreq -file cert-file -storepass password

# Sign certificate
openssl x509 -req -CA ca-cert -CAkey ca-key -in cert-file -out cert-signed -days 365 -CAcreateserial

# Import CA into keystore
keytool -keystore kafka.server.keystore.jks -alias CARoot -import -file ca-cert -storepass password -noprompt

# Import signed certificate into keystore
keytool -keystore kafka.server.keystore.jks -alias kafka -import -file cert-signed -storepass password -noprompt

# Create truststore
keytool -keystore kafka.server.truststore.jks -alias CARoot -import -file ca-cert -storepass password -noprompt

# Create credential files
echo "password" > kafka_keystore_creds
echo "password" > kafka_ssl_key_creds
echo "password" > kafka_truststore_creds

cd ..

echo "🐳 Starting Docker containers..."
docker-compose up -d

echo "⏳ Waiting for KDC to be ready..."
sleep 30

echo "👤 Creating Kerberos principals and keytabs..."

# Create principals and keytabs
docker exec krb5-kdc kadmin.local -q "addprinc -pw password kafka/kafka.example.com@EXAMPLE.COM"
docker exec krb5-kdc kadmin.local -q "addprinc -pw password zookeeper/zookeeper.example.com@EXAMPLE.COM"  
docker exec krb5-kdc kadmin.local -q "addprinc -pw password ktranslate@EXAMPLE.COM"
docker exec krb5-kdc kadmin.local -q "addprinc -pw password kafka-ui@EXAMPLE.COM"

# Generate keytabs
docker exec krb5-kdc kadmin.local -q "ktadd -k /tmp/keytabs/kafka.keytab kafka/kafka.example.com@EXAMPLE.COM"
docker exec krb5-kdc kadmin.local -q "ktadd -k /tmp/keytabs/zookeeper.keytab zookeeper/zookeeper.example.com@EXAMPLE.COM"
docker exec krb5-kdc kadmin.local -q "ktadd -k /tmp/keytabs/ktranslate.keytab ktranslate@EXAMPLE.COM"
docker exec krb5-kdc kadmin.local -q "ktadd -k /tmp/keytabs/kafka-ui.keytab kafka-ui@EXAMPLE.COM"

echo "🔄 Restarting services with keytabs..."
docker-compose restart kafka zookeeper

echo "⏳ Waiting for services to be ready..."
sleep 45

echo "✅ Testing Kafka connectivity..."

# Test topic creation
docker exec kafka-broker kafka-topics --bootstrap-server kafka.example.com:9094 \
  --command-config /etc/kafka/kafka_client.properties \
  --create --topic ktranslate-test --partitions 3 --replication-factor 1 || true

echo ""
echo "🎉 Environment setup complete!"
echo ""
echo "📊 Access points:"
echo "- Kafka (SASL_SSL): kafka.example.com:9094"
echo "- Kafka (PLAINTEXT): localhost:9092"
echo "- Kafka UI: http://localhost:8080"
echo "- KDC: localhost:88"
echo ""
echo "🔑 Kerberos details:"
echo "- Realm: EXAMPLE.COM"
echo "- Principal: ktranslate@EXAMPLE.COM"
echo "- Keytab: ./kerberos/ktranslate.keytab"
echo ""
echo "📁 Generated files:"
echo "- Kerberos config: ./kerberos/krb5.conf"
echo "- SSL certificates: ./ssl/"
echo "- Keytabs: ./kerberos/*.keytab"
