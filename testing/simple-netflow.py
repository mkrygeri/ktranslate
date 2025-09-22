#!/usr/bin/env python3
"""
Simple NetFlow v5 Test Packet Generator
Creates minimal but valid NetFlow v5 packets for testing ktranslate
"""

import socket
import struct
import time
import sys

def send_simple_netflow():
    """Send a simple NetFlow v5 packet with one flow record"""
    
    # NetFlow v5 Header (24 bytes)
    version = 5
    count = 1  # one flow record
    sys_uptime = int(time.time() * 1000) & 0xFFFFFFFF
    unix_secs = int(time.time())
    unix_nsecs = 0
    flow_sequence = 1
    engine_type = 0
    engine_id = 0
    sampling_interval = 0
    
    header = struct.pack('!HHIIIIBBH',
                        version, count, sys_uptime, unix_secs, unix_nsecs,
                        flow_sequence, engine_type, engine_id, sampling_interval)
    
    # NetFlow v5 Flow Record (48 bytes)
    srcaddr = struct.unpack('!I', socket.inet_aton('192.168.1.100'))[0]
    dstaddr = struct.unpack('!I', socket.inet_aton('10.0.0.200'))[0]
    nexthop = 0
    input_snmp = 1
    output_snmp = 2
    dPkts = 10
    dOctets = 1500
    first = sys_uptime
    last = sys_uptime + 1000
    srcport = 80
    dstport = 443
    pad1 = 0
    tcp_flags = 0x18
    prot = 6
    tos = 0
    src_as = 65001
    dst_as = 65002
    src_mask = 24
    dst_mask = 24
    pad2 = 0
    
    record = struct.pack('!IIIHHIIHHHHBBHHBB',
                        srcaddr, dstaddr, nexthop,
                        input_snmp, output_snmp,
                        dPkts, dOctets,
                        first, last,
                        srcport, dstport,
                        pad1, tcp_flags, prot, tos,
                        src_as, dst_as,
                        src_mask, dst_mask)
    
    # Combine and send
    packet = header + record
    
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        sent = sock.sendto(packet, ('localhost', 9995))
        print(f"Sent NetFlow v5 packet: {sent} bytes")
        return sent
    except Exception as e:
        print(f"Error: {e}")
        return 0
    finally:
        sock.close()

if __name__ == '__main__':
    print("Sending simple NetFlow v5 test packet to ktranslate...")
    for i in range(3):
        send_simple_netflow()
        time.sleep(1)
    print("Done!")