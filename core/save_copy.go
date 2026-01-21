package core

import (
	"archive/tar"
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

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func (c *CLab) copySavedConfigs(ctx context.Context, node clabnodes.Node, dstRoot string) error {
	provider, ok := node.(clabnodes.SavedConfigPathProvider)
	if !ok {
		return nil
	}

	paths := provider.SavedConfigPaths()
	if len(paths) == 0 {
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

	timestamp := time.Now().UTC().Format("060102_150405")

	var errs []error
	for _, src := range paths {
		if src == "" {
			continue
		}

		if err := copySavedPath(ctx, src, nodeCfg.LabDir, nodeDstDir, timestamp); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func copySavedPath(
	ctx context.Context,
	src,
	nodeLabDir,
	dstNodeDir,
	timestamp string,
) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %q: %w", src, err)
	}

	rel := relPathWithinDir(src, nodeLabDir)
	dstLatest := filepath.Join(dstNodeDir, rel)

	if info.IsDir() {
		if err := os.RemoveAll(dstLatest); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %q: %w", dstLatest, err)
		}

		if err := copyDir(ctx, src, dstLatest); err != nil {
			return fmt.Errorf("copy dir %q: %w", src, err)
		}

		archivePath := timestampedDirArchivePath(dstLatest, timestamp)
		if err := archiveDir(src, archivePath); err != nil {
			return fmt.Errorf("archive dir %q: %w", src, err)
		}

		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dstLatest), clabconstants.PermissionsDirDefault); err != nil {
		return fmt.Errorf("create dst dir for %q: %w", dstLatest, err)
	}

	if err := clabutils.CopyFile(ctx, src, dstLatest, info.Mode().Perm()); err != nil {
		return fmt.Errorf("copy file %q: %w", src, err)
	}

	tsPath, compress := timestampedFilePath(dstLatest, timestamp)
	if compress {
		if err := gzipFile(src, tsPath, clabconstants.PermissionsFileDefault); err != nil {
			return fmt.Errorf("compress file %q: %w", src, err)
		}
	} else {
		if err := clabutils.CopyFile(ctx, src, tsPath, clabconstants.PermissionsFileDefault); err != nil {
			return fmt.Errorf("copy compressed file %q: %w", src, err)
		}
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

func copyDir(ctx context.Context, src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(target), clabconstants.PermissionsDirDefault); err != nil {
				return err
			}

			return os.Symlink(linkTarget, target)
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type %q", path)
		}

		return clabutils.CopyFile(ctx, path, target, info.Mode().Perm())
	})
}

func archiveDir(srcDir, dstTarGz string) error {
	out, cleanup, err := clabutils.CreateFileWithPermissions(
		dstTarGz,
		clabconstants.PermissionsFileDefault,
	)
	if err != nil {
		return err
	}
	defer cleanup()

	gzipWriter := gzip.NewWriter(out)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	base := filepath.Base(srcDir)

	return filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		name := filepath.Join(base, rel)
		if rel == "." {
			name = base
		}

		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(name)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(tarWriter, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
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

func timestampedDirArchivePath(dstLatest, timestamp string) string {
	dir := filepath.Dir(dstLatest)
	base := filepath.Base(dstLatest)

	return filepath.Join(dir, fmt.Sprintf("%s-%s.tar.gz", base, timestamp))
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
