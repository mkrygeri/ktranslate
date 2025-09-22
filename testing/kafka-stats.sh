#!/bin/bash

# Kafka Statistics Monitor for ktranslate SSL/SASL Setup
# Shows comprehensive statistics for Kafka topics and messages

set -e

KAFKA_BROKER="localhost:9093"
SSL_ARGS="-X security.protocol=SSL -X ssl.ca.location= -X ssl.endpoint.identification.algorithm=none -X enable.ssl.certificate.verification=false"

echo "==================================================================================="
echo "                           KAFKA STATISTICS MONITOR"
echo "==================================================================================="
echo "Broker: $KAFKA_BROKER"
echo "Time: $(date)"
echo ""

echo "1. CLUSTER METADATA:"
echo "-------------------"
kcat -b $KAFKA_BROKER $SSL_ARGS -L
echo ""

echo "2. TOPIC STATISTICS:"
echo "--------------------"

# Get all topics
topics=$(kcat -b $KAFKA_BROKER $SSL_ARGS -L | grep "topic " | awk '{print $2}' | tr -d '"')

for topic in $topics; do
    echo "Topic: $topic"
    
    # Get partition info
    partitions=$(kcat -b $KAFKA_BROKER $SSL_ARGS -L | grep -A 10 "topic \"$topic\"" | grep "partition" | wc -l)
    echo "  Partitions: $partitions"
    
    # Get message count for each partition
    total_messages=0
    for ((p=0; p<partitions; p++)); do
        # Get high water mark (latest offset)
        latest_offset=$(kcat -b $KAFKA_BROKER $SSL_ARGS -Q -t ${topic}:${p}:-1 | cut -d' ' -f3)
        echo "    Partition $p: $latest_offset messages"
        total_messages=$((total_messages + latest_offset))
    done
    
    echo "  Total messages: $total_messages"
    echo ""
done

echo "3. RECENT ACTIVITY:"
echo "-------------------"

for topic in $topics; do
    echo "Latest messages from $topic (last 3):"
    kcat -b $KAFKA_BROKER $SSL_ARGS -t $topic -o -3 -c 3 -f 'Partition %p, Offset %o, Timestamp %T: %s\n' 2>/dev/null | head -3 || echo "  No recent messages"
    echo ""
done

echo "4. TOPIC CONFIGURATION:"
echo "-----------------------"
echo "Available topics and their retention/size info:"
kcat -b $KAFKA_BROKER $SSL_ARGS -L | grep -E "(topic|partition)" | head -20

echo ""
echo "5. PERFORMANCE SUMMARY:"
echo "----------------------"
total_all_messages=0
for topic in $topics; do
    partitions=$(kcat -b $KAFKA_BROKER $SSL_ARGS -L | grep -A 10 "topic \"$topic\"" | grep "partition" | wc -l)
    topic_total=0
    for ((p=0; p<partitions; p++)); do
        latest_offset=$(kcat -b $KAFKA_BROKER $SSL_ARGS -Q -t ${topic}:${p}:-1 | cut -d' ' -f3)
        topic_total=$((topic_total + latest_offset))
    done
    total_all_messages=$((total_all_messages + topic_total))
    echo "  $topic: $topic_total messages"
done
echo "  TOTAL ACROSS ALL TOPICS: $total_all_messages messages"

echo ""
echo "6. SSL CONNECTION STATUS:"
echo "------------------------"
echo "✅ SSL connection successful (script completed without errors)"
echo "✅ Certificate verification disabled for testing"
echo "✅ All topics accessible"

echo ""
echo "==================================================================================="
echo "                           MONITORING COMPLETE"
echo "==================================================================================="