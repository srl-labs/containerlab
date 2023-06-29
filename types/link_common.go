package types

type EndpointRaw struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
	Mac   string `yaml:"mac"`
}

// func extractHostNodeInterfaceData(lc *LinkConfig, specialEPIndex int) (host string, hostIf string, node string, nodeIf string) {
// 	// the index of the node is the specialEndpointIndex +1  modulo 2
// 	nodeindex := (specialEPIndex + 1) % 2

// 	hostData := strings.SplitN(lc.Endpoints[specialEPIndex], ":", 2)
// 	nodeData := strings.SplitN(lc.Endpoints[nodeindex], ":", 2)

// 	host = hostData[0]
// 	hostIf = hostData[1]
// 	node = nodeData[0]
// 	nodeIf = nodeData[1]

// 	return host, hostIf, node, nodeIf
// }
