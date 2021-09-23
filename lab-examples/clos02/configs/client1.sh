#!/bin/bash

cat > /etc/network/interfaces << EOF
auto eth1

iface eth1 inet static
  address 10.0.0.25
  netmask 255.255.255.254

iface eth1 inet6 static
  address 1000:10:0:0::25
  netmask 127
  pre-up echo 0 > /proc/sys/net/ipv6/conf/eth1/accept_ra
EOF

ifup eth1
