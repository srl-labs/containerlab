package srl

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/srl-labs/containerlab/types"
)

type mac struct {
	MAC string
}

// genMac returns a struct with a generated MAC address string to use in SR Linux
// topology file.
func genMac(cfg *types.NodeConfig) mac {
	// Generated MAC address conforms to the following addressing scheme
	// first byte  - `1a` - fixed for easy identification of SRL Mac addresses
	// second byte - random, to distinguish projects
	// third byte  - index of the topology node

	projID, _ := rand.Int(rand.Reader, big.NewInt(256))
	macPrefix := fmt.Sprintf("1a:%02x", projID)

	// labs up to 256 nodes are supported, behaviour is undefined when more nodes are defined
	m := fmt.Sprintf("%s:%02x:00:00:00", macPrefix, cfg.Index%256)

	// set system Mac in NodeConfig
	cfg.MacAddress = m

	return mac{
		MAC: m,
	}
}
