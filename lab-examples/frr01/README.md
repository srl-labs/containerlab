# Simple OSPF lab using FRR

This lab example consists of three FRR routers connected in a ring topology. Each router has one PC connected to it. The routers are running OSPF. The PCs have static routes.

IP Addresses
* **PC1:** 192.168.11.2/24
* **PC2:** 192.168.12.2/24
* **PC3:** 192.168.13.2/24

This is also an example of how to pre-configure lab nodes on "linux" node types in Containerlab.

To start this lab, run the *clab deploy* command.

The lab configuration is documented in detail at: https://www.brianlinkletter.com/2021/05/use-containerlab-to-emulate-open-source-routers/
