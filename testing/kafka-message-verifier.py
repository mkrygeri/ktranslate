#!/usr/bin/env python3
"""
Kafka Message Verifier - Connects to Kafka and dumps topic messages to console
Supports SSL, SASL_SSL, and PLAINTEXT configurations for ktranslate verification
"""

import sys
import argparse
import json
import time
from datetime import datetime
from kafka import KafkaConsumer
from kafka.errors import KafkaError
import ssl


def create_ssl_context():
    """Create SSL context for certificate verification"""
    ssl_context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH)
    ssl_context.check_hostname = False
    ssl_context.verify_mode = ssl.CERT_NONE
    return ssl_context


def create_consumer(config):
    """Create Kafka consumer based on configuration"""
    consumer_config = {
        'bootstrap_servers': config['brokers'],
        'auto_offset_reset': 'latest',  # Start from newest messages
        'enable_auto_commit': True,
        'group_id': f'ktranslate-verifier-{int(time.time())}',
        'value_deserializer': lambda x: x.decode('utf-8') if x else None,
        'consumer_timeout_ms': 5000,  # 5 second timeout
    }
    
    if config['security_protocol'] == 'SSL':
        print(f"[INFO] Configuring SSL connection to {config['brokers']}")
        consumer_config.update({
            'security_protocol': 'SSL',
            'ssl_context': create_ssl_context(),
        })
        
    elif config['security_protocol'] == 'SASL_SSL':
        print(f"[INFO] Configuring SASL_SSL connection to {config['brokers']}")
        consumer_config.update({
            'security_protocol': 'SASL_SSL',
            'sasl_mechanism': 'GSSAPI',
            'ssl_context': create_ssl_context(),
        })
        
    elif config['security_protocol'] == 'PLAINTEXT':
        print(f"[INFO] Configuring PLAINTEXT connection to {config['brokers']}")
        consumer_config['security_protocol'] = 'PLAINTEXT'
    
    return KafkaConsumer(**consumer_config)


def format_message(message, show_metadata=True):
    """Format message for display"""
    timestamp = datetime.fromtimestamp(message.timestamp / 1000.0) if message.timestamp else "N/A"
    
    output = []
    if show_metadata:
        output.append(f"--- Message ---")
        output.append(f"Topic: {message.topic}")
        output.append(f"Partition: {message.partition}")
        output.append(f"Offset: {message.offset}")
        output.append(f"Timestamp: {timestamp}")
        output.append(f"Key: {message.key}")
        output.append(f"Headers: {message.headers}")
        output.append(f"Value Length: {len(message.value) if message.value else 0} bytes")
        output.append("--- Content ---")
    
    # Try to parse as JSON for pretty printing
    try:
        if message.value:
            parsed = json.loads(message.value)
            output.append(json.dumps(parsed, indent=2))
        else:
            output.append("(empty message)")
    except json.JSONDecodeError:
        # Not JSON, print as plain text
        output.append(message.value if message.value else "(empty message)")
    
    if show_metadata:
        output.append("=" * 50)
    
    return "\n".join(output)


def main():
    parser = argparse.ArgumentParser(description='Kafka Message Verifier for ktranslate')
    parser.add_argument('--topic', required=True, help='Kafka topic to consume from')
    parser.add_argument('--brokers', default='localhost:9092', help='Kafka brokers (default: localhost:9092)')
    parser.add_argument('--security', choices=['PLAINTEXT', 'SSL', 'SASL_SSL'], 
                       default='PLAINTEXT', help='Security protocol (default: PLAINTEXT)')
    parser.add_argument('--ssl-port', type=int, default=9093, help='SSL port (default: 9093)')
    parser.add_argument('--count', type=int, help='Maximum number of messages to consume')
    parser.add_argument('--timeout', type=int, default=30, help='Timeout in seconds (default: 30)')
    parser.add_argument('--compact', action='store_true', help='Show compact output without metadata')
    parser.add_argument('--follow', action='store_true', help='Keep consuming messages (like tail -f)')
    
    args = parser.parse_args()
    
    # Adjust broker port for SSL/SASL_SSL
    if args.security in ['SSL', 'SASL_SSL'] and ':' not in args.brokers:
        brokers = f"{args.brokers}:{args.ssl_port}"
    else:
        brokers = args.brokers
    
    config = {
        'brokers': [brokers],
        'security_protocol': args.security,
    }
    
    print(f"[INFO] Kafka Message Verifier starting...")
    print(f"[INFO] Topic: {args.topic}")
    print(f"[INFO] Brokers: {brokers}")
    print(f"[INFO] Security: {args.security}")
    print(f"[INFO] Timeout: {args.timeout}s")
    if args.count:
        print(f"[INFO] Max messages: {args.count}")
    if args.follow:
        print(f"[INFO] Following mode: will keep consuming until interrupted")
    print("-" * 50)
    
    try:
        consumer = create_consumer(config)
        consumer.subscribe([args.topic])
        
        message_count = 0
        start_time = time.time()
        
        print(f"[INFO] Consuming from topic '{args.topic}'...")
        print(f"[INFO] Waiting for messages... (Press Ctrl+C to stop)")
        
        while True:
            try:
                # Poll for messages
                message_batch = consumer.poll(timeout_ms=1000)
                
                if not message_batch:
                    # Check timeout
                    if not args.follow and (time.time() - start_time) > args.timeout:
                        print(f"[INFO] Timeout reached ({args.timeout}s), no messages received")
                        break
                    continue
                
                # Process messages
                for topic_partition, messages in message_batch.items():
                    for message in messages:
                        message_count += 1
                        
                        print(format_message(message, show_metadata=not args.compact))
                        
                        # Check count limit
                        if args.count and message_count >= args.count:
                            print(f"[INFO] Reached message limit ({args.count})")
                            return
                        
                        # Flush output
                        sys.stdout.flush()
                
            except KeyboardInterrupt:
                print(f"\n[INFO] Interrupted by user")
                break
                
    except KafkaError as e:
        print(f"[ERROR] Kafka error: {e}")
        return 1
    except Exception as e:
        print(f"[ERROR] Unexpected error: {e}")
        return 1
    finally:
        try:
            consumer.close()
        except:
            pass
        
        print(f"\n[INFO] Consumed {message_count} messages total")
    
    return 0


if __name__ == '__main__':
    sys.exit(main())