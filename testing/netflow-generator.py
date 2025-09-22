#!/usr/bin/env python3
"""
Simple NetFlow v5 Generator for testing ktranslate
Generates basic NetFlow v5 packets and sends them to ktranslate
"""

import socket
import struct
import time
import sys
import argparse

def create_netflow_v5_header(count=1):
    """Create NetFlow v5 header"""
    version = 5
    flow_count = count
    sys_uptime = int(time.time() * 1000) & 0xFFFFFFFF  # milliseconds
    unix_seconds = int(time.time())
    unix_nanoseconds = 0
    flow_sequence = 0
    engine_type = 0
    engine_id = 0
    sampling_interval = 0
    
    return struct.pack('!HHIIIIBBH',
                      version,           # 2 bytes
                      flow_count,        # 2 bytes  
                      sys_uptime,        # 4 bytes
                      unix_seconds,      # 4 bytes
                      unix_nanoseconds,  # 4 bytes
                      flow_sequence,     # 4 bytes
                      engine_type,       # 1 byte
                      engine_id,         # 1 byte
                      sampling_interval) # 2 bytes

def create_netflow_v5_record(src_ip, dst_ip, src_port, dst_port, proto):
    """Create a NetFlow v5 record (48 bytes)"""
    
    # Convert IP addresses to integers
    def ip_to_int(ip):
        return struct.unpack('!I', socket.inet_aton(ip))[0]
    
    srcaddr = ip_to_int(src_ip)
    dstaddr = ip_to_int(dst_ip)
    nexthop = ip_to_int('0.0.0.0')
    
    input_if = 1
    output_if = 2
    packets = 10
    octets = 1500
    first = int(time.time() * 1000) & 0xFFFFFFFF
    last = first + 1000
    tcp_flags = 0x18  # PSH+ACK
    tos = 0
    src_as = 65001
    dst_as = 65002
    src_mask = 24
    dst_mask = 24
    pad2 = 0
    
    # NetFlow v5 record format (48 bytes total):
    return struct.pack('!IIIHHIIIIHHBBBBHHBBH',
                      srcaddr,     # 4 bytes - source IP
                      dstaddr,     # 4 bytes - dest IP  
                      nexthop,     # 4 bytes - next hop
                      input_if,    # 2 bytes - input interface
                      output_if,   # 2 bytes - output interface
                      packets,     # 4 bytes - packet count
                      octets,      # 4 bytes - byte count
                      first,       # 4 bytes - first packet time
                      last,        # 4 bytes - last packet time
                      src_port,    # 2 bytes - source port
                      dst_port,    # 2 bytes - dest port
                      pad2,        # 1 byte - padding
                      tcp_flags,   # 1 byte - TCP flags
                      proto,       # 1 byte - protocol
                      tos,         # 1 byte - type of service
                      src_as,      # 2 bytes - source AS
                      dst_as,      # 2 bytes - dest AS
                      src_mask,    # 1 byte - source mask
                      dst_mask,    # 1 byte - dest mask
                      0)           # 2 bytes - final padding

def send_netflow_packet(host='192.168.2.203', port=2055, flow_count=1):
    """Send NetFlow v5 packet to ktranslate"""
    
    # Maximum records per packet (to stay under UDP limit)
    MAX_RECORDS_PER_PACKET = 28  # Conservative limit
    
    if flow_count <= MAX_RECORDS_PER_PACKET:
        # Single packet
        return _send_single_packet(host, port, flow_count, 0)
    else:
        # Multiple packets
        total_sent = 0
        records_sent = 0
        packet_num = 0
        
        while records_sent < flow_count:
            remaining = flow_count - records_sent
            records_this_packet = min(remaining, MAX_RECORDS_PER_PACKET)
            
            sent = _send_single_packet(host, port, records_this_packet, records_sent)
            total_sent += sent
            records_sent += records_this_packet
            packet_num += 1
            
            print(f"  Sub-packet {packet_num}: Sent {sent} bytes ({records_this_packet} records)")
            
        return total_sent

def _send_single_packet(host, port, flow_count, start_offset=0):
    """Send a single NetFlow v5 packet"""
    # Create header
    header = create_netflow_v5_header(flow_count)
    
    # Create flow records
    records = b''
    for i in range(start_offset, start_offset + flow_count):
        # Vary the IPs slightly for each record (keep within valid ranges)
        src_ip = f'192.168.{1 + (i // 254)}.{1 + (i % 254)}'
        dst_ip = f'10.0.{(i // 254) % 256}.{1 + (i % 254)}'
        src_port = (1024 + i) % 65536  # Keep in valid port range
        dst_port = (8000 + i) % 65536  # Keep in valid port range
        proto = 6  # TCP protocol
        
        records += create_netflow_v5_record(src_ip, dst_ip, src_port, dst_port, proto)
    
    # Combine header and records
    packet = header + records
    
    # Send UDP packet
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        sent = sock.sendto(packet, (host, port))
        return sent
    finally:
        sock.close()

def main():
    parser = argparse.ArgumentParser(description='NetFlow v5 Generator for ktranslate testing')
    parser.add_argument('--host', default='localhost', help='Target host (default: localhost)')
    parser.add_argument('--port', type=int, default=9995, help='Target port (default: 9995)')
    parser.add_argument('--count', type=int, default=1, help='Number of flow records per packet (default: 1)')
    parser.add_argument('--packets', type=int, default=1, help='Number of packets to send (default: 1)')
    parser.add_argument('--interval', type=float, default=1.0, help='Interval between packets in seconds (default: 1.0)')
    
    args = parser.parse_args()
    
    print(f"NetFlow v5 Generator")
    print(f"Target: {args.host}:{args.port}")
    print(f"Flow records per packet: {args.count}")
    print(f"Packets to send: {args.packets}")
    print(f"Interval: {args.interval}s")
    print("-" * 40)
    
    for packet_num in range(args.packets):
        try:
            sent_bytes = send_netflow_packet(args.host, args.port, args.count)
            print(f"Packet {packet_num + 1}: Sent {sent_bytes} bytes ({args.count} flow records)")
            
            if packet_num < args.packets - 1:  # Don't sleep after the last packet
                time.sleep(args.interval)
                
        except Exception as e:
            print(f"Error sending packet {packet_num + 1}: {e}")
            return 1
    
    print(f"\nSent {args.packets} NetFlow v5 packets successfully!")
    return 0

if __name__ == '__main__':
    sys.exit(main())