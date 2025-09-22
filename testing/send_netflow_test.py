#!/usr/bin/env python3
import socket
import struct
import time

def send_netflow_packet():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    
    # NetFlow v5 header + 1 flow record  
    header = struct.pack('!HHIIIIBBH',
        5,      # version
        1,      # count
        int(time.time() * 1000) % (2**32),  # sys_uptime (keep in 32-bit range)
        int(time.time()),         # unix_secs
        0,      # unix_nsecs
        0,      # flow_sequence
        0,      # engine_type
        0,      # engine_id
        0       # sampling_interval
    )
    
    # Flow record
    flow = struct.pack('!IIIIHHHIIHHBBBBHHBB',
        0xC0A80101,  # srcaddr (192.168.1.1)
        0xC0A80102,  # dstaddr (192.168.1.2)
        0x00000000,  # nexthop
        1,           # input
        2,           # output
        1000,        # packets
        64000,       # bytes
        int(time.time()) - 60,  # first
        int(time.time()),       # last
        80,          # srcport 
        443,         # dstport 
        0,           # pad1
        0x18,        # tcp_flags
        6,           # prot (TCP)
        0,           # tos
        65001 % 65536,       # src_as (keep in 16-bit range)
        65002 % 65536,       # dst_as (keep in 16-bit range)
        24,          # src_mask
        24           # dst_mask
    )
    
    packet = header + flow
    sock.sendto(packet, ('127.0.0.1', 9995))
    print("📊 NetFlow packet sent to ktranslate on port 9995")
    sock.close()

if __name__ == "__main__":
    send_netflow_packet()