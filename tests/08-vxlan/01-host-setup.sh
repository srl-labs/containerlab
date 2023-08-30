#!/bin/bash

CNAME=vxlep

# create a docker container
docker run -d --name ${CNAME} alpine:latest sleep infinity
# add proper iproute2 package
docker exec ${CNAME} apk add iproute2

# populate /var/run/netns with docker container network namespace
pid=$(docker inspect -f '{{.State.Pid}}' ${CNAME})
sudo ln -sf /proc/$pid/ns/net /var/run/netns/${CNAME}

# add a veth pair between host and the above started container
# this veth will act as the underlay link for the vxlan
sudo ip l add link1a mtu 9100 type veth peer name link1b mtu 9100
sudo ip a add dev link1a 192.168.66.0/31
sudo ip l set link1a up
sudo ip l set link1b netns ${CNAME}
sudo ip netns exec ${CNAME} ip a add dev link1b 192.168.66.1/31
sudo ip netns exec ${CNAME} ip l set link1b up

# add the vxlan link to the container
sudo ip netns exec ${CNAME} ip l add vxlan100 type vxlan id 100 remote 192.168.66.0 dstport 5555
sudo ip netns exec ${CNAME} ip a add dev vxlan100 192.168.67.1/24
sudo ip netns exec ${CNAME} ip l set vxlan100 up

# cleanup the network namespace link
sudo rm -f /var/run/netns/${CNAME}