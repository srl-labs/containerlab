#!/usr/bin/env python3
"""
DNS Proxy for ContainerLab Tailscale integration with 1:1 NAT.
Rewrites DNS queries and responses for bidirectional translation:
- PTR queries: Rewrites NAT IP -> real IP (in query name)
- A record responses: Rewrites real IP -> NAT IP (in response data)
"""

import socket
import struct
import sys
import select
import subprocess

# Configuration - these will be replaced by Go template
LISTEN_PORT = {{.ListenPort}}
BACKEND_PORT = {{.BackendPort}}
MGMT_SUBNET = '{{.MgmtSubnet}}'  # Real management subnet
NAT_SUBNET = '{{.NatSubnet}}'   # NAT subnet advertised to Tailscale

# Parse subnet to get base IP and mask
def parse_subnet(subnet):
    ip_str, prefix = subnet.split('/')
    ip = struct.unpack('!I', socket.inet_aton(ip_str))[0]
    mask = (0xFFFFFFFF << (32 - int(prefix))) & 0xFFFFFFFF
    return ip & mask, mask

MGMT_BASE, MGMT_MASK = parse_subnet(MGMT_SUBNET)
NAT_BASE, NAT_MASK = parse_subnet(NAT_SUBNET)

def translate_ip_to_nat(ip_str):
    """Translate real mgmt IP to NAT IP"""
    try:
        ip = struct.unpack('!I', socket.inet_aton(ip_str))[0]
        if (ip & MGMT_MASK) == MGMT_BASE:
            offset = ip - MGMT_BASE
            nat_ip = NAT_BASE + offset
            return socket.inet_ntoa(struct.pack('!I', nat_ip))
    except:
        pass
    return ip_str

def translate_ip_to_real(ip_str):
    """Translate NAT IP to real mgmt IP"""
    try:
        ip = struct.unpack('!I', socket.inet_aton(ip_str))[0]
        if (ip & NAT_MASK) == NAT_BASE:
            offset = ip - NAT_BASE
            real_ip = MGMT_BASE + offset
            return socket.inet_ntoa(struct.pack('!I', real_ip))
    except:
        pass
    return ip_str

def parse_ptr_query(name_bytes, start_pos):
    """
    Parse PTR query name and extract IP address.
    Returns (ip_address, end_pos) or (None, end_pos) if not a valid PTR query.
    PTR format: <reversed-ip>.in-addr.arpa
    Example: 254.200.20.172.in-addr.arpa for 172.20.200.254
    """
    labels = []
    pos = start_pos
    
    while pos < len(name_bytes):
        length = name_bytes[pos]
        if length == 0:
            pos += 1
            break
        if length >= 0xC0:  # Compression pointer
            pos += 2
            break
        pos += 1
        label = name_bytes[pos:pos+length].decode('ascii', errors='ignore')
        labels.append(label)
        pos += length
    
    # Check if this is a PTR query (ends with in-addr.arpa)
    if len(labels) >= 6 and labels[-2:] == ['in-addr', 'arpa']:
        # Extract IP octets (reversed)
        octets = labels[-6:-2]
        if len(octets) == 4 and all(o.isdigit() and 0 <= int(o) <= 255 for o in octets):
            # Reverse to get actual IP
            ip_str = '.'.join(reversed(octets))
            return ip_str, pos
    
    return None, pos

def encode_dns_name(name):
    """Encode domain name into DNS wire format"""
    parts = name.split('.')
    result = bytearray()
    for part in parts:
        if part:
            result.append(len(part))
            result.extend(part.encode('ascii'))
    result.append(0)  # Null terminator
    return bytes(result)

def is_from_tailscale(addr):
    """Check if query is from Tailscale (100.x.x.x or fd7a:)"""
    ip = addr[0]
    # Tailscale IPv4 uses 100.x.x.x range
    if ip.startswith('100.'):
        return True
    # Tailscale IPv6 uses fd7a: prefix
    if ip.startswith('fd7a:'):
        return True
    return False

def rewrite_ptr_query(data, from_tailscale):
    """Rewrite PTR query to translate NAT IP to real IP in query name"""
    if not from_tailscale or len(data) < 12:
        return data
    
    try:
        # Parse DNS header
        query_id, flags, qdcount, ancount, nscount, arcount = struct.unpack('!HHHHHH', data[:12])
        
        # Only process queries (not responses)
        if flags & 0x8000:  # QR bit set = response
            return data
        
        if qdcount == 0:
            return data
        
        # Parse first question to check if it's a PTR query
        ptr_ip, end_pos = parse_ptr_query(data, 12)
        
        if ptr_ip:
            # Translate NAT IP to real IP
            real_ip = translate_ip_to_real(ptr_ip)
            if real_ip != ptr_ip:
                # Reconstruct query with real IP
                # PTR format: reverse octets + .in-addr.arpa
                real_octets = real_ip.split('.')
                ptr_name = '.'.join(reversed(real_octets)) + '.in-addr.arpa'
                
                # Build new query
                new_query = bytearray(data[:12])  # Keep header
                new_query.extend(encode_dns_name(ptr_name))  # New PTR name
                new_query.extend(data[end_pos:])  # Keep type, class, and rest
                
                print(f"Rewrote PTR query: {ptr_ip} -> {real_ip}", file=sys.stderr, flush=True)
                return bytes(new_query)
        
        return data
    except Exception as e:
        print(f"Error rewriting PTR query: {e}", file=sys.stderr, flush=True)
        return data

def rewrite_dns_response(data, from_tailscale):
    """Rewrite IP addresses in DNS response if needed"""
    if not from_tailscale or len(data) < 12:
        return data
    
    try:
        # Parse DNS header
        query_id, flags, qdcount, ancount, nscount, arcount = struct.unpack('!HHHHHH', data[:12])
        
        # Only process responses with answers
        if ancount == 0:
            return data
        
        response = bytearray(data)
        pos = 12
        
        # Skip questions
        for _ in range(qdcount):
            while pos < len(response):
                length = response[pos]
                if length == 0:
                    pos += 5  # null + type + class
                    break
                if length >= 0xC0:  # Compression pointer
                    pos += 6  # pointer + type + class
                    break
                pos += length + 1
        
        # Process answers
        for _ in range(ancount):
            if pos >= len(response):
                break
                
            # Skip name (handle compression)
            if response[pos] >= 0xC0:
                pos += 2
            else:
                while pos < len(response) and response[pos] != 0:
                    pos += response[pos] + 1
                pos += 1
            
            if pos + 10 > len(response):
                break
                
            rtype, rclass, ttl, rdlength = struct.unpack('!HHIH', response[pos:pos+10])
            pos += 10
            
            # Rewrite A records (type 1, IPv4)
            if rtype == 1 and rdlength == 4:
                ip_bytes = response[pos:pos+4]
                ip_str = socket.inet_ntoa(bytes(ip_bytes))
                new_ip_str = translate_ip_to_nat(ip_str)
                if new_ip_str != ip_str:
                    new_ip_bytes = socket.inet_aton(new_ip_str)
                    response[pos:pos+4] = new_ip_bytes
                    print(f"Rewrote {ip_str} -> {new_ip_str}", file=sys.stderr, flush=True)
            
            pos += rdlength
        
        return bytes(response)
    except Exception as e:
        print(f"Error rewriting DNS response: {e}", file=sys.stderr, flush=True)
        return data

def main():
    print(f"Starting DNS proxy on port {LISTEN_PORT}, forwarding to 127.0.0.1:{BACKEND_PORT}", flush=True)
    print(f"Mgmt subnet: {MGMT_SUBNET}, NAT subnet: {NAT_SUBNET}", flush=True)
    
    # Get container's management IP address
    try:
        # Get the container's IP on eth0 (management interface)
        result = subprocess.run(['hostname', '-i'], capture_output=True, text=True)
        container_ip = result.stdout.strip().split()[0]  # First IP
        print(f"Binding to {container_ip}:{LISTEN_PORT}", flush=True)
        bind_addr = container_ip
    except:
        print(f"Could not determine container IP, binding to 0.0.0.0", flush=True)
        bind_addr = '0.0.0.0'
    
    # Create UDP socket for listening
    listen_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    listen_sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listen_sock.bind((bind_addr, LISTEN_PORT))
    
    # Create UDP socket for backend
    backend_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    
    # Map to track queries: backend_sock -> (client_addr, from_tailscale)
    pending_queries = {}
    
    print("DNS proxy ready", flush=True)
    
    while True:
        readable, _, _ = select.select([listen_sock, backend_sock], [], [], 1.0)
        
        for sock in readable:
            if sock == listen_sock:
                # Query from client
                data, client_addr = listen_sock.recvfrom(512)
                from_ts = is_from_tailscale(client_addr)
                
                # Rewrite PTR queries if from Tailscale (NAT IP -> real IP)
                if from_ts:
                    data = rewrite_ptr_query(data, from_ts)
                
                # Forward to backend
                backend_sock.sendto(data, ('127.0.0.1', BACKEND_PORT))
                
                # Store client info for response
                query_id = struct.unpack('!H', data[:2])[0]
                pending_queries[query_id] = (client_addr, from_ts)
                
            elif sock == backend_sock:
                # Response from backend
                data, _ = backend_sock.recvfrom(512)
                
                query_id = struct.unpack('!H', data[:2])[0]
                if query_id in pending_queries:
                    client_addr, from_ts = pending_queries.pop(query_id)
                    
                    # Rewrite A record IPs if from Tailscale (real IP -> NAT IP)
                    data = rewrite_dns_response(data, from_ts)
                    
                    # Send back to client
                    listen_sock.sendto(data, client_addr)

if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        print("DNS proxy stopped", flush=True)
        sys.exit(0)
