/interface bridge add name=lo0
/interface ethernet set ether1 network-type=point-to-point
/ip address add address=3.3.3.3/32 interface=lo0
