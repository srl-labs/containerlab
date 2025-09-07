package core

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcoredependency_manager "github.com/srl-labs/containerlab/core/dependency_manager"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const topoFromLabListTimeout = 30 * time.Second

type ClabOption func(c *CLab) error

// WithLabOwner sets the owner label for all nodes in the lab.
// Only users in the clab_admins group can set a custom owner.
func WithLabOwner(owner string) ClabOption {
	return func(c *CLab) error {
		currentUser, err := user.Current()
		if err != nil {
			log.Warn("Failed to get current user when trying to set the custom lab owner", "error", err)
			return nil
		}

		if isClabAdmin, err := clabutils.UserInUnixGroup(currentUser.Username,
			"clab_admins"); err == nil && isClabAdmin {
			c.customOwner = owner
		} else if owner != "" {
			log.Warn("Only users in clab_admins group can set custom owner. Using current user as owner.")
		}

		return nil
	}
}

func WithTimeout(dur time.Duration) ClabOption {
	return func(c *CLab) error {
		if dur <= 0 {
			return errors.New("zero or negative timeouts are not allowed")
		}

		c.timeout = dur

		return nil
	}
}

// WithTopologyName sets the name of the lab/topology
// to the provided string.
func WithTopologyName(n string) ClabOption {
	return func(c *CLab) error {
		c.Config.Name = n

		return nil
	}
}

// WithSkippedBindsPathsCheck skips the binds paths checks.
func WithSkippedBindsPathsCheck() ClabOption {
	return func(c *CLab) error {
		c.checkBindsPaths = false

		return nil
	}
}

// WithManagementNetworkName sets the name of the
// management network that is to be used.
func WithManagementNetworkName(n string) ClabOption {
	return func(c *CLab) error {
		c.Config.Mgmt.Network = n

		return nil
	}
}

// WithManagementIpv4Subnet defined the IPv4 subnet
// that will be used for the mgmt network.
func WithManagementIpv4Subnet(s string) ClabOption {
	return func(c *CLab) error {
		c.Config.Mgmt.IPv4Subnet = s

		return nil
	}
}

// WithManagementIpv6Subnet defined the IPv6 subnet
// that will be used for the mgmt network.
func WithManagementIpv6Subnet(s string) ClabOption {
	return func(c *CLab) error {
		c.Config.Mgmt.IPv6Subnet = s

		return nil
	}
}

// WithDependencyManager adds Dependency Manager.
func WithDependencyManager(dm clabcoredependency_manager.DependencyManager) ClabOption {
	return func(c *CLab) error {
		c.dependencyManager = dm

		return nil
	}
}

// WithDebug sets debug mode.
func WithDebug(debug bool) ClabOption {
	return func(c *CLab) error {
		c.Config.Debug = debug

		return nil
	}
}

// WithRuntime option sets a container runtime to be used by containerlab.
func WithRuntime(name string, rtconfig *clabruntime.RuntimeConfig) ClabOption {
	return func(c *CLab) error {
		name, rInit, err := RuntimeInitializer(name)
		if err != nil {
			return err
		}

		c.globalRuntimeName = name

		r := rInit()

		log.Debugf("Running runtime.Init with params %+v and %+v", rtconfig, c.Config.Mgmt)

		err = r.Init(
			clabruntime.WithConfig(rtconfig),
			clabruntime.WithMgmtNet(c.Config.Mgmt),
		)
		if err != nil {
			return fmt.Errorf("failed to init the container runtime: %v", err)
		}

		c.Runtimes[name] = r

		log.Debugf("initialized a runtime with params %+v", r)

		return nil
	}
}

func WithKeepMgmtNet() ClabOption {
	return func(c *CLab) error {
		c.globalRuntime().WithKeepMgmtNet()

		return nil
	}
}

func WithTopoPath(path, varsFile string) ClabOption {
	return func(c *CLab) error {
		file, err := c.ProcessTopoPath(path)
		if err != nil {
			return err
		}

		if err := c.LoadTopologyFromFile(file, varsFile); err != nil {
			return fmt.Errorf("failed to read topology file: %v", err)
		}

		return c.initMgmtNetwork()
	}
}

// WithTopoBackup creates a backup of the topology file.
func WithTopoBackup(path string) ClabOption {
	return func(c *CLab) error {
		// create a backup file for the topology file
		backupFPath := c.TopoPaths.TopologyBakFileAbsPath()

		err := clabutils.CopyFile(
			context.Background(),
			path,
			backupFPath,
			clabconstants.PermissionsFileDefault,
		)
		if err != nil {
			log.Warn("Could not create topology backup", "topology path", path,
				"backup path", backupFPath, "error", err)
		}

		return nil
	}
}

// WithTopoFromLab loads the topology file path based on a running lab name.
// The lab name is used to look up the container labels of a running lab and
// derive the topology file location. It falls back to WithTopoPath once the
// topology path is discovered.
func WithTopoFromLab(labName string) ClabOption {
	return func(c *CLab) error {
		if labName == "" {
			return fmt.Errorf("lab name is required to derive topology path")
		}

		ctx, cancel := context.WithTimeout(context.Background(), topoFromLabListTimeout)
		defer cancel()

		filter := []*clabtypes.GenericFilter{
			{
				FilterType: "label",
				Field:      clabconstants.Containerlab,
				Operator:   "=",
				Match:      labName,
			},
		}

		containers, err := c.globalRuntime().ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers for lab '%s': %w", labName, err)
		}

		if len(containers) == 0 {
			return fmt.Errorf("lab '%s' not found - no running containers", labName)
		}

		topoFile := containers[0].Labels[clabconstants.TopoFile]
		if topoFile == "" {
			return fmt.Errorf("could not determine topology file from container labels")
		}

		// Verify topology file exists and is accessible
		if !clabutils.FileOrDirExists(topoFile) {
			return fmt.Errorf(
				"topology file '%s' referenced by lab '%s' does not exist or is not accessible",
				topoFile,
				labName,
			)
		}

		log.Debugf("found topology file for lab %s: %s", labName, topoFile)

		return WithTopoPath(topoFile, "")(c)
	}
}

// WithNodeFilter option sets a filter for nodes to be deployed.
// A filter is a list of node names to be deployed,
// names are provided exactly as they are listed in the topology file.
// Since this is altering the clab.config.Topology.[Nodes,Links] it must only
// be called after WithTopoFile.
func WithNodeFilter(nodeFilter []string) ClabOption {
	return func(c *CLab) error {
		return c.filterClabNodes(nodeFilter)
	}
}
