# Packet capture in c9s

The c9s connectivity sidecar provides the supported packet-capture path for lab
interfaces, including captures that preserve 802.1Q tags. Follow the
[c9s packet capture guide](https://c9s.run/docs/guides/lab-operations#packet-capture)
for the current command and options.

The command streams standard pcap data through the Kubernetes API. It does not
require access to a cluster worker, but the local kube identity must have
`pods/exec` permission and be able to reach the API server. Save the stream to a
local file as shown in that guide, or pipe it to a local Wireshark process.
