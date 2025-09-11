#!/bin/bash

set -e

echo "🔐 Setting up Complete Kerberos Infrastructure for ktranslate Testing"
echo "=================================================================="

echo ""
echo "📋 Infrastructure Overview:"
echo "• Kerberos KDC (Key Distribution Center)"
echo "• Kafka with SASL_PLAINTEXT and SASL_SSL listeners"
echo "• Service principals: kafka/localhost@EXAMPLE.COM, ktranslate@EXAMPLE.COM"
echo "• SSL certificates for encrypted transport"
echo "• Complete authentication testing environment"

echo ""
echo "🚀 Step 1: Stop existing containers and clean up"
docker-compose down || true
docker-compose -f docker-compose-kerberos.yml down || true

echo ""
echo "🔑 Step 2: Set up SSL certificates"
./setup-ssl.sh

echo ""
echo "📝 Step 3: Copy krb5.conf to proper location"
cp kerberos/krb5-test.conf kerberos/krb5.conf

echo ""
echo "🏗️  Step 4: Start Kerberos infrastructure"
echo "Starting KDC first..."
docker-compose -f docker-compose-kerberos.yml up -d kerberos-kdc

echo "Waiting for KDC to initialize..."
sleep 15

echo "Starting Kafka with Kerberos support..."
docker-compose -f docker-compose-kerberos.yml up -d kafka-broker

echo "Starting remaining services..."
docker-compose -f docker-compose-kerberos.yml up -d

echo ""
echo "⏱️  Step 5: Wait for services to be ready"
sleep 20

echo ""
echo "🧪 Step 6: Verify Kerberos setup"
echo "Checking KDC status..."
docker-compose -f docker-compose-kerberos.yml exec kerberos-client bash -c "
    apt-get update -qq && apt-get install -y krb5-user -qq
    cp /kerberos-setup/krb5.conf /etc/krb5.conf
    
    echo '🔍 Testing Kerberos authentication:'
    echo 'Available principals:'
    echo '• ktranslate@EXAMPLE.COM (password: ktranslate123)'
    echo '• kafka/localhost@EXAMPLE.COM (password: kafka123)'
    
    echo ''
    echo 'Testing keytab authentication:'
    kinit -kt /kerberos-setup/ktranslate.keytab ktranslate@EXAMPLE.COM
    klist
" || echo "KDC setup in progress..."

echo ""
echo "📊 Step 7: Create Kerberos-enabled ktranslate configuration"
cat > ktranslate-kerberos-full.yaml << 'EOF'
sinks:
  - kafka
format: json
kafkasink:
  bootstrapservers: "localhost:9093"
  topic: "ktranslate-kerberos-test"
  securityprotocol: "SASL_PLAINTEXT"
  saslmechanism: "GSSAPI"
  kerberosservicename: "kafka"
  kerberosrealm: "EXAMPLE.COM"
  kerberosconfigpath: "./kerberos/krb5.conf"
  kerberoskeytabpath: "./kerberos/ktranslate.keytab"
  kerberosprincipal: "ktranslate@EXAMPLE.COM"
  kerberosdiablepaaxfast: false
  compression: "gzip"
  requiredacks: 1
  retrymax: 3
  flushfrequency: 100
rollup:
  maxmemoryMB: 128
  maxkeys: 5000
  emergencycleanup: true
server:
  loglevel: "debug"
  logtostdout: true
EOF

echo ""
echo "🎉 KERBEROS INFRASTRUCTURE READY!"
echo "================================="
echo ""
echo "✅ Services Running:"
docker-compose -f docker-compose-kerberos.yml ps

echo ""
echo "🔐 Kerberos Testing Commands:"
echo "1. Test authentication:"
echo "   docker-compose -f docker-compose-kerberos.yml exec kerberos-client kinit -kt /kerberos-setup/ktranslate.keytab ktranslate@EXAMPLE.COM"
echo ""
echo "2. List tickets:"
echo "   docker-compose -f docker-compose-kerberos.yml exec kerberos-client klist"
echo ""
echo "3. Test ktranslate with Kerberos:"
echo "   ../bin/ktranslate -config=./ktranslate-kerberos-full.yaml"

echo ""
echo "🚀 Next: Run the complete Kerberos authentication test!"
echo "   ./test-kerberos-complete.sh"
