package sros

import (
	"crypto/rand"
	"fmt"
	"math/big"

	clabtypes "github.com/srl-labs/containerlab/types"
)

// genMac returns a struct with a generated MAC address string to use in SR-OS Node.
func genMac(cfg *clabtypes.NodeConfig) string {
	// Generated MAC address conforms to the following addressing scheme
	// first byte  - `1c` - fixed for easy identification of SR-OS Mac addresses
	// second byte - random, to distinguish projects
	// third byte  - index of the topology node

	projID, _ := rand.Int(rand.Reader, big.NewInt(256))
	macPrefix := fmt.Sprintf("1c:%02x", projID)

	// labs up to 256 nodes are supported, behavior is undefined when more nodes are defined
	m := fmt.Sprintf("%s:%02x:00:00:00", macPrefix, cfg.Index%256)

	// set system Mac in NodeConfig
	cfg.MacAddress = m

	return m
}
