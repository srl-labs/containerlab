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
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func (c *CLab) copySavedConfig(ctx context.Context, node clabnodes.Node, dstRoot string) error {
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

	if err := copyLabDirContents(ctx, nodeCfg.LabDir, nodeDstDir, timestamp); err != nil {
		return err
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
