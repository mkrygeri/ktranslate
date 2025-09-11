#!/bin/bash

set -e

echo "🚀 Setting up Kafka + SSL Testing Environment"
echo "=============================================="

# Create necessary directories
mkdir -p ssl logs

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

echo "⏳ Waiting for services to be ready..."
sleep 30

echo "✅ Testing Kafka connectivity..."

# Test topic creation
docker exec kafka-broker kafka-topics --bootstrap-server localhost:9092 \
  --create --topic ktranslate-test --partitions 3 --replication-factor 1 || true

echo ""
echo "🎉 Environment setup complete!"
echo ""
echo "📊 Access points:"
echo "- Kafka (SSL): kafka.example.com:9094"
echo "- Kafka (PLAINTEXT): localhost:9092"
echo "- Kafka UI: http://localhost:8080"
echo ""
echo "📁 Generated files:"
echo "- SSL certificates: ./ssl/"
