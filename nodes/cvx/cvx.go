package cvx

import (
	"context"
	"fmt"
	"regexp"

	"github.com/charmbracelet/log"
	"github.com/distribution/reference"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabtypes "github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"cvx", "cumulus_cvx"}

var memoryReqs = map[string]string{
	"4.3.0": "512MB",
	"4.4.0": "768MB",
}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	r.Register(kindnames, func() clabnodes.Node {
		return new(cvx)
	}, nil)
}

type cvx struct {
	clabnodes.DefaultNode
}

func (c *cvx) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	c.DefaultNode = *clabnodes.NewDefaultNode(c)

	c.Cfg = cfg
	for _, o := range opts {
		o(c)
	}

	imageRef, err := reference.ParseNormalizedNamed(cfg.Image)
	if err != nil {
		return fmt.Errorf("failed to parse image ref %q: %w", cfg.Image, err)
	}

	// if Memory is not statically set, apply the defaults
	if cfg.Memory == "" {
		tag := ""
		if tagged, ok := imageRef.(reference.Tagged); ok {
			tag = tagged.Tag()
		}

		mem, ok := memoryReqs[tag]
		cfg.Memory = mem

		// by default setting the limit to 768MB
		if !ok {
			cfg.Memory = "768MB"
		}
	}

	return nil
}

func (c *cvx) Deploy(ctx context.Context, _ *clabnodes.DeployParams) error {
	cID, err := c.Runtime.CreateContainer(ctx, c.Cfg)
	if err != nil {
		return err
	}
	_, err = c.Runtime.StartContainer(ctx, cID, c)
	if err != nil {
		return err
	}

	c.SetState(clabnodesstate.Deployed)

	return nil
}

func (c *cvx) PostDeploy(_ context.Context, _ *clabnodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for cvx '%s' node", c.Cfg.ShortName)
	return nil
}

func (c *cvx) GetImages(_ context.Context) map[string]string {
	images := make(map[string]string)
	images[clabnodes.ImageKey] = c.Cfg.Image
	return images
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (c *cvx) CheckInterfaceName() error {
	// allow swpX interface names
	// https://regex101.com/r/SV0k1J/1
	ifRe := regexp.MustCompile(`swp[\d.]+$`)
	for _, e := range c.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf(
				"%q interface name %q doesn't match the required pattern. It should be named as swpX, where X is >=0",
				c.Cfg.ShortName,
				e.GetIfaceName(),
			)
		}
	}

	return nil
}
