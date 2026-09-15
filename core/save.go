package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

	if opts.copyDst != "" {
		resolvedDst, err := c.resolveCopyOutDst(opts.copyDst)
		if err != nil {
			return err
		}
		opts.copyDst = resolvedDst
	}

	var wg sync.WaitGroup

	wg.Add(len(c.Nodes))

	for _, node := range c.Nodes {
		go func(node clabnodes.Node) {
			defer wg.Done()

			result, err := node.SaveConfig(ctx)
			if err != nil {
				log.Errorf("node %q save failed: %v", node.GetShortName(), err)
				return
			}

			if opts.copyDst == "" {
				return
			}

			if err := c.copySavedConfig(ctx, result, node, opts.copyDst); err != nil {
				log.Errorf("node %q save copy failed: %v", node.GetShortName(), err)
			}
		}(node)
	}

	wg.Wait()

	return nil
}

func (c *CLab) resolveCopyOutDst(dst string) (string, error) {
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

// copySavedConfig copies the saved configuration file from the path in the SaveConfigResult
// to the user-specified destination directory with a timestamp in the filename.
// A symlink with the original filename is created (or updated) to point to the
// latest timestamped copy.
//
// The destination layout is:
//
//	<dstRoot>/<nodeName>/config-060102_150405.json   # timestamped copy
//	<dstRoot>/<nodeName>/config.json                 # symlink -> timestamped copy
func (c *CLab) copySavedConfig(
	ctx context.Context,
	result *clabnodes.SaveConfigResult,
	node clabnodes.Node,
	dstRoot string,
) error {
	if result == nil || result.ConfigPath == "" {
		log.Debug("no saved config path reported, skipping copy", "node", node.GetShortName())
		return nil
	}

	nodeCfg := node.Config()
	if nodeCfg == nil {
		return fmt.Errorf("node config missing")
	}

	nodeDstDir := filepath.Join(dstRoot, nodeCfg.ShortName)
	if err := os.MkdirAll(nodeDstDir, clabconstants.PermissionsDirDefault); err != nil {
		return fmt.Errorf("failed to create save dst node dir %q: %w", nodeDstDir, err)
	}

	fileName := filepath.Base(result.ConfigPath)
	tsFileName := timestampedFileName(fileName, time.Now())

	tsPath := filepath.Join(nodeDstDir, tsFileName)
	if err := clabutils.CopyFile(ctx, result.ConfigPath, tsPath,
		clabconstants.PermissionsFileDefault); err != nil {
		return fmt.Errorf("failed to copy saved config from %q to %q: %w",
			result.ConfigPath, tsPath, err)
	}

	// Create or update a symlink with the original filename pointing to the
	// timestamped copy. Remove any existing file/symlink first.
	linkPath := filepath.Join(nodeDstDir, fileName)
	_ = os.Remove(linkPath)
	if err := os.Symlink(tsFileName, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink %q -> %q: %w", linkPath, tsFileName, err)
	}

	log.Info(
		"copied saved config",
		"node", nodeCfg.ShortName,
		"src", result.ConfigPath,
		"dst", tsPath,
	)

	return nil
}

// timestampedFileName inserts a timestamp before the file extension.
// e.g. "config.json" with ts 2026-02-06 23:59:00 UTC -> "config-260206_235900.json".
// Files without an extension get the timestamp appended: "startup-config" ->
// "startup-config-260206_235900".
func timestampedFileName(name string, t time.Time) string {
	ts := t.UTC().Format("060102_150405")
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	return base + "-" + ts + ext
}
