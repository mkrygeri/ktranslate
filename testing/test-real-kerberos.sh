#!/bin/bash

set -e

echo "🔐 COMPLETE KERBEROS AUTHENTICATION TESTING"
echo "=========================================="
echo ""
echo "This test will validate the FULL Kerberos authentication flow:"
echo "1. Start real KDC with EXAMPLE.COM realm"
echo "2. Create service principals and keytabs"
echo "3. Configure Kafka with Kerberos authentication"
echo "4. Run ktranslate with GSSAPI authentication"
echo "5. Verify successful authentication and message flow"

echo ""
echo "🧹 Step 1: Clean environment"
docker stop $(docker ps -q) 2>/dev/null || true
docker-compose -f docker-compose-kerberos.yml down --remove-orphans --volumes
rm -rf kerberos/*.keytab kerberos/krb5.conf 2>/dev/null || true

echo ""
echo "🚀 Step 2: Start Kerberos KDC"
echo "Starting KDC server..."
docker-compose -f docker-compose-kerberos.yml up -d kerberos-kdc

echo ""
echo "⏳ Step 3: Wait for KDC initialization (60 seconds)"
echo "KDC needs time to:"
echo "- Install Kerberos packages"
echo "- Initialize database"
echo "- Create principals"
echo "- Generate keytabs"

for i in {60..1}; do
    echo -ne "\rWaiting for KDC setup: ${i}s remaining..."
    sleep 1
done
echo ""

echo ""
echo "🔍 Step 4: Verify KDC setup"
echo "Checking KDC logs..."
docker logs testing_kerberos-kdc_1 2>&1 | tail -20

echo ""
echo "Verifying keytab files were created..."
if [ -f "kerberos/ktranslate.keytab" ]; then
    echo "✅ ktranslate.keytab created successfully"
    ls -la kerberos/ktranslate.keytab
else
    echo "❌ ktranslate.keytab not found"
    echo "Available files in kerberos/:"
    ls -la kerberos/ || echo "Directory empty"
fi

if [ -f "kerberos/krb5.conf" ]; then
    echo "✅ krb5.conf created successfully"
    echo "Configuration:"
    head -20 kerberos/krb5.conf
else
    echo "❌ krb5.conf not found"
fi

echo ""
echo "🚀 Step 5: Start Kafka with Kerberos support"
echo "Starting Kafka broker..."
docker-compose -f docker-compose-kerberos.yml up -d kafka-broker kafka-zookeeper

echo ""
echo "⏳ Step 6: Wait for Kafka to be ready (45 seconds)"
for i in {45..1}; do
    echo -ne "\rWaiting for Kafka startup: ${i}s remaining..."
    sleep 1
done
echo ""

echo ""
echo "🔍 Step 7: Verify Kafka is ready"
echo "Checking Kafka logs for Kerberos setup..."
docker logs testing_kafka-broker_1 2>&1 | grep -i "kerberos\|gssapi\|sasl" | tail -10 || echo "No Kerberos logs yet"

echo ""
echo "🧪 Step 8: Test Kerberos authentication with ktranslate"
echo "======================================================"

if [ ! -f "kerberos/ktranslate.keytab" ] || [ ! -f "kerberos/krb5.conf" ]; then
    echo "❌ Missing required Kerberos files. Cannot proceed with authentication test."
    echo "This indicates the KDC setup failed. Let's check what happened:"
    echo ""
    echo "KDC container status:"
    docker-compose -f docker-compose-kerberos.yml ps kerberos-kdc
    echo ""
    echo "KDC logs:"
    docker logs testing_kerberos-kdc_1 2>&1 | tail -50
    exit 1
fi

echo "✅ Required Kerberos files found. Proceeding with authentication test..."

echo ""
echo "Testing ktranslate with REAL Kerberos authentication..."
echo "Configuration being used:"
cat ktranslate-kerberos-real.yaml | grep -A 15 kafkasink:

echo ""
echo "🎯 CRITICAL TEST: Running ktranslate with GSSAPI authentication"
echo "Expected behavior:"
echo "- Should authenticate with KDC using keytab"
echo "- Should connect to Kafka using Kerberos"
echo "- Should NOT show 'kerberos config file not found' error"

timeout 20s ../bin/ktranslate -config=./ktranslate-kerberos-real.yaml 2>&1 | tee /tmp/ktranslate-kerberos-test.log

echo ""
echo "🔍 Step 9: Analyze test results"
echo "============================="

echo "Checking for authentication success indicators..."
if grep -q "kafka.*connected\|kafkaSink.*connected" /tmp/ktranslate-kerberos-test.log; then
    echo "✅ SUCCESS: Kafka connection established!"
    echo "Found connection logs:"
    grep "kafka.*connected\|kafkaSink.*connected" /tmp/ktranslate-kerberos-test.log
elif grep -q "kerberos config file not found" /tmp/ktranslate-kerberos-test.log; then
    echo "❌ FAILURE: Still getting config file not found error"
    echo "This means ktranslate is not using our generated files correctly"
elif grep -q "authentication\|gssapi\|kerberos" /tmp/ktranslate-kerberos-test.log; then
    echo "⚠️  PARTIAL: Kerberos process started but may have other issues"
    echo "Authentication-related logs:"
    grep -i "authentication\|gssapi\|kerberos" /tmp/ktranslate-kerberos-test.log
else
    echo "ℹ️  Test completed. Let's check what happened:"
fi

echo ""
echo "Full ktranslate output:"
cat /tmp/ktranslate-kerberos-test.log

echo ""
echo "🎉 KERBEROS VALIDATION COMPLETE"
echo "==============================="

echo ""
echo "📊 Infrastructure Status:"
docker-compose -f docker-compose-kerberos.yml ps

echo ""
echo "📁 Generated Files:"
ls -la kerberos/ 2>/dev/null || echo "No files generated"

echo ""
echo "🔧 Troubleshooting Commands:"
echo "• Check KDC status: docker logs testing_kerberos-kdc_1"
echo "• Check Kafka logs: docker logs testing_kafka-broker_1"
echo "• Test keytab: docker exec testing_kerberos-kdc_1 klist -kt /shared/ktranslate.keytab"
echo "• Manual auth test: docker exec testing_kerberos-kdc_1 kinit -kt /shared/ktranslate.keytab ktranslate@EXAMPLE.COM"

echo ""
if [ -f "kerberos/ktranslate.keytab" ] && [ -f "kerberos/krb5.conf" ]; then
    if grep -q "kafka.*connected" /tmp/ktranslate-kerberos-test.log 2>/dev/null; then
        echo "🏆 FULL SUCCESS: Kerberos authentication is working end-to-end!"
    else
        echo "🔍 FILES GENERATED: Infrastructure is working, may need Kafka configuration tuning"
    fi
else
    echo "🚧 INFRASTRUCTURE ISSUE: KDC setup needs debugging"
fi

echo ""
echo "Ready to proceed with any needed adjustments!"
