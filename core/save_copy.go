package core

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func (c *CLab) copySavedConfig(
	ctx context.Context,
	node clabnodes.Node,
	dstRoot string,
	result *clabtypes.SaveConfigResult,
) error {
	nodeCfg := node.Config()
	if nodeCfg == nil {
		return fmt.Errorf("node config missing")
	}

	if nodeCfg.LabDir == "" {
		return fmt.Errorf("node lab directory is empty")
	}

	nodeDstDir := filepath.Join(dstRoot, nodeCfg.ShortName)
	if err := os.MkdirAll(nodeDstDir, clabconstants.PermissionsDirDefault); err != nil {
		return fmt.Errorf("failed to create save dst node dir %q: %w", nodeDstDir, err)
	}

	timestamp := time.Now().UTC().Format("060102_150405")

	if result == nil {
		return fmt.Errorf("saved config result missing")
	}

	switch {
	case len(result.Payload) > 0:
		if err := writeSavedConfigPayload(ctx, nodeDstDir, result.PayloadName, result.Payload, timestamp); err != nil {
			return err
		}
	case len(result.FilePaths) > 0:
		if err := copySavedConfigFiles(ctx, nodeCfg.LabDir, nodeDstDir, timestamp, result.FilePaths); err != nil {
			return err
		}
	default:
		return fmt.Errorf("no saved config artifacts found in %q", nodeCfg.LabDir)
	}

	log.Info(
		"copied saved configs",
		"node",
		nodeCfg.ShortName,
		"dst",
		nodeDstDir,
	)

	return nil
}

func (c *CLab) discoverSavedConfigResult(
	node clabnodes.Node,
	saveStart time.Time,
) (*clabtypes.SaveConfigResult, error) {
	nodeCfg := node.Config()
	if nodeCfg == nil {
		return nil, fmt.Errorf("node config missing")
	}

	if nodeCfg.LabDir == "" {
		return nil, fmt.Errorf("node lab directory is empty")
	}

	cutoff := saveStart.Add(-2 * time.Second)
	paths, err := findChangedFiles(nodeCfg.LabDir, cutoff)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		paths = append(paths, fallbackSavedConfigPaths(nodeCfg)...)
	}

	paths = uniqueSorted(paths)

	return &clabtypes.SaveConfigResult{
		FilePaths: paths,
	}, nil
}

func writeSavedConfigPayload(
	ctx context.Context,
	dstDir string,
	name string,
	payload []byte,
	timestamp string,
) error {
	if name == "" {
		name = "saved-config"
	}

	target := filepath.Join(dstDir, name)
	out, cleanup, err := clabutils.CreateFileWithPermissions(target, clabconstants.PermissionsFileDefault)
	if err != nil {
		return err
	}
	defer cleanup()

	if _, err := out.Write(payload); err != nil {
		return err
	}

	if err := out.Sync(); err != nil {
		return err
	}

	tsPath, compress := timestampedFilePath(target, timestamp)
	if compress {
		return gzipFile(target, tsPath, clabconstants.PermissionsFileDefault)
	}

	return clabutils.CopyFile(ctx, target, tsPath, clabconstants.PermissionsFileDefault)
}

func copySavedConfigFiles(
	ctx context.Context,
	labDir string,
	dstDir string,
	timestamp string,
	filePaths []string,
) error {
	var errs []error
	seen := map[string]struct{}{}

	for _, filePath := range filePaths {
		absPath := filePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(labDir, filePath)
		}

		rel, err := relPathWithinDirStrict(absPath, labDir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}

		info, err := os.Lstat(absPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		target := filepath.Join(dstDir, rel)
		if info.IsDir() {
			if err := copyLabDirContents(ctx, absPath, target, timestamp); err != nil {
				errs = append(errs, err)
			}
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(absPath)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(target), clabconstants.PermissionsDirDefault); err != nil {
				errs = append(errs, err)
				continue
			}

			_ = os.RemoveAll(target)
			if err := os.Symlink(linkTarget, target); err != nil {
				errs = append(errs, err)
			}
			continue
		}

		if !info.Mode().IsRegular() {
			errs = append(errs, fmt.Errorf("unsupported file type %q", absPath))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), clabconstants.PermissionsDirDefault); err != nil {
			errs = append(errs, err)
			continue
		}

		if err := clabutils.CopyFile(ctx, absPath, target, info.Mode().Perm()); err != nil {
			errs = append(errs, err)
			continue
		}

		tsPath, compress := timestampedFilePath(target, timestamp)
		if compress {
			if err := gzipFile(absPath, tsPath, clabconstants.PermissionsFileDefault); err != nil {
				errs = append(errs, err)
			}
		} else {
			if err := clabutils.CopyFile(ctx, absPath, tsPath, clabconstants.PermissionsFileDefault); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func findChangedFiles(root string, cutoff time.Time) ([]string, error) {
	var matches []string
	var errs []error

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			errs = append(errs, err)
			return nil
		}

		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			return nil
		}

		matches = append(matches, path)
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return matches, nil
}

func fallbackSavedConfigPaths(nodeCfg *clabtypes.NodeConfig) []string {
	var paths []string

	if nodeCfg.ResStartupConfig != "" && clabutils.FileExists(nodeCfg.ResStartupConfig) {
		if _, err := relPathWithinDirStrict(nodeCfg.ResStartupConfig, nodeCfg.LabDir); err == nil {
			paths = append(paths, nodeCfg.ResStartupConfig)
		}
	}

	srlPath := filepath.Join(nodeCfg.LabDir, "config", "config.json")
	if clabutils.FileExists(srlPath) {
		paths = append(paths, srlPath)
	}

	return paths
}

func uniqueSorted(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	sort.Strings(paths)

	out := paths[:0]
	var last string
	for i, p := range paths {
		if i == 0 || p != last {
			out = append(out, p)
			last = p
		}
	}

	return out
}

func relPathWithinDirStrict(path, base string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("base directory is empty")
	}

	rel, err := filepath.Rel(base, path)
	if err != nil {
		return "", err
	}

	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside base directory %q", path, base)
	}

	return rel, nil
}

func copyLabDirContents(ctx context.Context, srcDir, dstDir, timestamp string) error {
	info, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("stat %q: %w", srcDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory at %q", srcDir)
	}

	var errs []error
	walkErr := filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil
		}

		rel := relPathWithinDir(path, srcDir)
		if rel == "." {
			return nil
		}

		target := filepath.Join(dstDir, rel)
		info, err := os.Lstat(path)
		if err != nil {
			errs = append(errs, err)
			return nil
		}

		if entry.IsDir() {
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				errs = append(errs, err)
			}
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				errs = append(errs, err)
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(target), clabconstants.PermissionsDirDefault); err != nil {
				errs = append(errs, err)
				return nil
			}

			_ = os.RemoveAll(target)
			if err := os.Symlink(linkTarget, target); err != nil {
				errs = append(errs, err)
			}
			return nil
		}

		if !info.Mode().IsRegular() {
			errs = append(errs, fmt.Errorf("unsupported file type %q", path))
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(target), clabconstants.PermissionsDirDefault); err != nil {
			errs = append(errs, err)
			return nil
		}

		if err := clabutils.CopyFile(ctx, path, target, info.Mode().Perm()); err != nil {
			errs = append(errs, err)
			return nil
		}

		tsPath, compress := timestampedFilePath(target, timestamp)
		if compress {
			if err := gzipFile(path, tsPath, clabconstants.PermissionsFileDefault); err != nil {
				errs = append(errs, err)
			}
		} else {
			if err := clabutils.CopyFile(ctx, path, tsPath, clabconstants.PermissionsFileDefault); err != nil {
				errs = append(errs, err)
			}
		}

		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func relPathWithinDir(path, base string) string {
	if base == "" {
		return filepath.Base(path)
	}

	rel, err := filepath.Rel(base, path)
	if err != nil {
		return filepath.Base(path)
	}

	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Base(path)
	}

	return rel
}

func gzipFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, cleanup, err := clabutils.CreateFileWithPermissions(dst, mode)
	if err != nil {
		return err
	}
	defer cleanup()

	gzipWriter := gzip.NewWriter(out)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, in)
	return err
}

func timestampedFilePath(dstLatest, timestamp string) (string, bool) {
	dir := filepath.Dir(dstLatest)
	name := filepath.Base(dstLatest)

	base, ext, alreadyCompressed := splitNameAndExt(name)
	if alreadyCompressed {
		return filepath.Join(dir, fmt.Sprintf("%s-%s%s", base, timestamp, ext)), false
	}

	return filepath.Join(dir, fmt.Sprintf("%s-%s%s.gz", base, timestamp, ext)), true
}

func splitNameAndExt(name string) (string, string, bool) {
	lower := strings.ToLower(name)

	switch {
	case strings.HasSuffix(lower, ".tgz"):
		return strings.TrimSuffix(name, ".tgz"), ".tgz", true
	case strings.HasSuffix(lower, ".gz"):
		base := strings.TrimSuffix(name, ".gz")
		ext := filepath.Ext(base)
		if ext == "" {
			return base, ".gz", true
		}
		return strings.TrimSuffix(base, ext), ext + ".gz", true
	case strings.HasSuffix(lower, ".bz2"):
		base := strings.TrimSuffix(name, ".bz2")
		ext := filepath.Ext(base)
		if ext == "" {
			return base, ".bz2", true
		}
		return strings.TrimSuffix(base, ext), ext + ".bz2", true
	case strings.HasSuffix(lower, ".xz"):
		base := strings.TrimSuffix(name, ".xz")
		ext := filepath.Ext(base)
		if ext == "" {
			return base, ".xz", true
		}
		return strings.TrimSuffix(base, ext), ext + ".xz", true
	case strings.HasSuffix(lower, ".zip"):
		base := strings.TrimSuffix(name, ".zip")
		ext := filepath.Ext(base)
		if ext == "" {
			return base, ".zip", true
		}
		return strings.TrimSuffix(base, ext), ext + ".zip", true
	default:
		ext := filepath.Ext(name)
		return strings.TrimSuffix(name, ext), ext, false
	}
}
