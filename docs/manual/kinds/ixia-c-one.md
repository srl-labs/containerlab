# Ixia-c-one

ixia-c-one is a re-packaged (as a single-container) flavor of multi-container application [ixia-c] (https://github.com/open-traffic-generator/ixia-c).
It is identified with `ixia-c-one` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `ixia-c-one` node.


## Managing ixia-c-one nodes
Ixia-c-one node launched with containerlab can be managed by generating stubs in a language of your choice from .proto files provided for Open Traffic Generator model at 
https://github.com/open-traffic-generator/models/releases
and then writing test programs in that language to congfigure and access the test ports.

In addition, easy to use python snappi and go snappi sdks are made available at 
https://github.com/open-traffic-generator/snappi/releases and  
https://github.com/open-traffic-generator/snappi/tree/main/gosnappi
Some initial steps are provided on how to download and start using the gosnappi module.
Please see the example for an actual step-by-step example of how to use it. 

=== "gosnappi"

ixia-c-one is configured by using Rest APIs. It can be configured using multiple language sdks.  
In this example, steps are provided to setup the test with gosnappi sdk.

Ensure go is installed on the system and confirm using `go version` that the version is at least `go1.17.3` . 
1. Set up a go module :
```bash
$ go mod init example/test
```

2. This test needs a gosnappi module , the version of which should match the ixia-c-one version being used.
The correct matching version can be found at https://github.com/open-traffic-generator/ixia-c/releases
```bash
go get github.com/open-traffic-generator/snappi/gosnappi@v0.7.18
```

3. Create any go file with suffix in the name as 'test', example l3_forward_test.go   
Create the go program to program the ixia-c-one ports using gosnappi APIs.  
Import the github.com/open-traffic-generator/snappi/gosnappi to get access to the gosnappi client sdk APIs.
Use these APIs to configure the test ports as per the needs of the test.  
For this refer to the example provided as well as at https://github.com/open-traffic-generator/snappi/tree/main/gosnappi

4. Now , run the test using the command :
```bash
go test -run=<Test Function> -v| tee out.log  
```

## Interfaces mapping
ixia-c-one container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* The other interfaces are the data interfaces which are created using same name as provided in the clab topology yaml file. 

When containerlab launches ixia-c-one node, it will set IPv4/6 addresses as assigned by docker to the `eth0` interface and ixia-c-one node will boot with that addresses configured.  

Data interfaces `eth1+` need to be configured with IP addressing manually if needed ( as in the Layer3 forwarding test example).
This is needed when the test port needs to reply to ARP/ND queries from the Device Under Test.

To configure an IPv4 address on any data link e.g. eth2 [Could be eth1 or eth3 as well, but never on eth0]:
```bash
docker exec -it clab-ixia-c-ixia-c-one bash
bash-5.1# bash set ipv4 eth2 2.2.2.2 24
```
To unset an IPv4 address on any data link e.g. eth2
```bash
bash-5.1# bash unset ipv4 eth2 2.2.2.2 24
```
To configure an IPv6 address on any data link e.g. eth1 :
```bash
docker exec -it clab-ixia-c-ixia-c-one bash
bash-5.1# bash set ipv6 eth1 11::1 64
```
To unset an IPv4 address on any data link e.g. eth2
```bash
bash-5.1# bash unset ipv6 eth1 11::1 64
```
## Features and options
The free version of ixia-c supports generation of L2 and L3 traffic to test forwarding of Ethernet, IPv4 and IPv6 traffic by switches and routers. For technical support and queries , please log requests at https://github.com/open-traffic-generator/ixia-c/issues or contact us @ https://ixia-c.slack.com/signup#/domain-signup .

The commercial version of ixia-c supports ARP/ND/Auto destination MAC resolution in data traffic, IPv4 and IPv6 BGP with IPv4 and IPv6 Routes and ISIS with IPv4 and IPv6 routes. Please contact Keysight support for further information regarding this if needed.

## Lab examples
The following labs feature ixia-c-one node:

- [Ixia-c-one and Arista cEOS](../../lab-examples/ixiacone-ceos.md)

## Known issues or limitations
1: For L3 traffic tests using the free version , there is no in-built support of ARP and ND.  
This can be worked around by manually setting IP address on the receive interface (as explained in Interfaces mapping section above) and by learning the MAC of the connected DUT using external means such as gnmi/ssh/reading it from CLI and using it when generating packets.  
This limitation will be removed in the ixia-c-one free version in future releases where it is planned to support ARP/ND Request and Reply for emulated interfaces.  

2: Every time a clab with an ixia-c-one node is removed, it leaves behind a peristent storage.  
If there are no other persistent unlinked storages on your system, you can remove it by removing all unlinked persistent storages by giving the command:
```bash
docker volume prune
```
If you wish to be very safe:
```bash
docker inspect clab-ixia-c-ixia-c-one  
...
"Mounts": [
            {
                "Type": "volume",
                "Name": "d1e87f85d3352bfb9dac3f8bac8eebee738503802cb9380966b5c4805bd791da",  #### get the volume name 
                "Source": "/var/lib/docker/volumes/d1e87f85d3352bfb9dac3f8bac8eebee738503802cb9380966b5c4805bd791da/_data",
                "Destination": "/var/lib/docker", 
...
docker volume remove d1e87f85d3352bfb9dac3f8bac8eebee738503802cb9380966b5c4805bd791da
```
[Note: This can be fixed handled in clab with one of the two approaches below:  
i) During docker run pass the --rm flag when starting the containers, or  
ii) During docker rm pass the -v flag when removing the containers.]

