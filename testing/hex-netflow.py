#!/usr/bin/env python3
"""
Send pre-built NetFlow v5 packet to ktranslate for testing
Uses a known-good NetFlow v5 packet in hex format
"""

import socket
import time

def send_netflow_hex():
    """Send a pre-built NetFlow v5 packet"""
    
    # This is a valid NetFlow v5 packet with header + 1 flow record
    # Header (24 bytes) + Flow Record (48 bytes) = 72 bytes total
    netflow_hex = (
        # NetFlow v5 Header (24 bytes)
        "0005"        # version (5)
        "0001"        # count (1 flow)
        "12345678"    # sys_uptime
        "62e42a00"    # unix_secs (current time approx)
        "00000000"    # unix_nsecs
        "00000001"    # flow_sequence
        "00"          # engine_type
        "01"          # engine_id
        "0000"        # sampling_interval
        
        # NetFlow v5 Flow Record (48 bytes)
        "c0a80164"    # srcaddr (192.168.1.100)
        "0a0000c8"    # dstaddr (10.0.0.200)
        "00000000"    # nexthop (0.0.0.0)
        "0001"        # input interface (1)
        "0002"        # output interface (2)
        "0000000a"    # packets (10)
        "000005dc"    # octets (1500)
        "12345678"    # first (sys_uptime)
        "12346a40"    # last (sys_uptime + 4000)
        "0050"        # srcport (80)
        "01bb"        # dstport (443)
        "00"          # pad1
        "18"          # tcp_flags (PSH+ACK)
        "06"          # protocol (TCP)
        "00"          # tos
        "fde9"        # src_as (65001)
        "fdea"        # dst_as (65002)
        "18"          # src_mask (24)
        "18"          # dst_mask (24)
        "0000"        # pad2
    )
    
    # Convert hex string to bytes
    packet = bytes.fromhex(netflow_hex)
    
    # Send UDP packet
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        sent = sock.sendto(packet, ('localhost', 9995))
        print(f"Sent NetFlow v5 packet: {sent} bytes (hex: {len(netflow_hex)//2} bytes)")
        return sent
    except Exception as e:
        print(f"Error: {e}")
        return 0
    finally:
        sock.close()

if __name__ == '__main__':
    print("Sending NetFlow v5 test packets to ktranslate...")
    for i in range(3):
        print(f"Packet {i+1}:")
        send_netflow_hex()
        time.sleep(1)
    print("Done! Check ktranslate logs and Kafka messages.")