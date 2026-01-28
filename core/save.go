package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func (c *CLab) Save(
	ctx context.Context,
	options ...SaveOption,
) error {
	opts := NewSaveOptions()
	for _, opt := range options {
		opt(opts)
	}

	err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return err
	}

	if opts.dst != "" {
		resolvedDst, err := c.resolveSaveDst(opts.dst)
		if err != nil {
			return err
		}
		opts.dst = resolvedDst
	}
	dst := opts.dst

	var wg sync.WaitGroup

	wg.Add(len(c.Nodes))

	for _, node := range c.Nodes {
		go func(node clabnodes.Node) {
			defer wg.Done()

			if err := node.SaveConfig(ctx); err != nil {
				log.Errorf("node %q save failed: %v", node.GetShortName(), err)
				return
			}

			if dst == "" {
				return
			}

			if err := c.copySavedConfigs(ctx, node, dst); err != nil {
				log.Errorf("node %q save copy failed: %v", node.GetShortName(), err)
			}
		}(node)
	}

	wg.Wait()

	return nil
}

func (c *CLab) resolveSaveDst(dst string) (string, error) {
	baseDir, err := os.Getwd()
	if err != nil || baseDir == "" {
		baseDir = c.TopoPaths.TopologyFileDir()
	}
	if baseDir == "" {
		return "", fmt.Errorf("failed to resolve save dst: current working directory is empty")
	}

	resolvedDst := clabutils.ResolvePath(dst, baseDir)
	labDir := c.TopoPaths.TopologyLabDir()
	labDirName := filepath.Base(labDir)
	if labDirName == "" || labDirName == "." {
		return "", fmt.Errorf("failed to resolve save dst: lab directory is empty")
	}

	dstLabDir := filepath.Join(resolvedDst, labDirName)
	if err := os.MkdirAll(dstLabDir, clabconstants.PermissionsDirDefault); err != nil {
		return "", fmt.Errorf("failed to create save dst directory %q: %w", dstLabDir, err)
	}

	return dstLabDir, nil
}
