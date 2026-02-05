package sros

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	srosVersionFilePath   = "/etc/sros-version"
	srosImageTitleLabel   = "org.opencontainers.image.title"
	srosImageTitle        = "srsim"
	srosImageVendorLabel  = "org.opencontainers.image.vendor"
	srosImageVendor       = "Nokia"
	srosImageVersionLabel = "org.opencontainers.image.version"

	// vrnetlabVersionLabel is set on vrnetlab-built images; nokia_srsim kind must not use such images.
	vrnetlabVersionLabel = "vrnetlab-version"
)

// SrosVersion represents an SR OS version as a set of fields.
type SrosVersion struct {
	Major string
	Minor string
	Build string
}

// RunningVersion gets the software version of the running node
// by executing the "cat /etc/sros-version" command
// and parsing the output.
func (n *sros) RunningVersion(ctx context.Context) (*SrosVersion, error) {
	cmd, _ := clabexec.NewExecCmdFromString(`cat /etc/sros-version`)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	log.Debug(
		"Extracted raw SR OS version",
		"node",
		n.Cfg.ShortName,
		"stdout",
		execResult.GetStdOutString(),
		"stderr",
		execResult.GetStdErrString(),
	)

	return n.parseVersionString(execResult.GetStdOutString()), nil
}

func (*sros) parseVersionString(s string) *SrosVersion {
	re := regexp.MustCompile(`v?(\d+)\.(\d+)\.([A-Za-z0-9]+)`)

	v := re.FindStringSubmatch(s)
	// 4 matches must be returned if all goes well
	if len(v) != 4 {
		// return all zeroes if failed to parse
		return &SrosVersion{"0", "0", "0"}
	}
	return &SrosVersion{v[1], v[2], v[3]}
}

// String returns a string representation of the version in a semver fashion (with leading v).
func (v *SrosVersion) String() string {
	return fmt.Sprintf("v%s.%s.%s", v.Major, v.Minor, v.Build)
}

// MajorMinorSemverString returns a string representation of the major.minor version with a leading
// v.
func (v *SrosVersion) MajorMinorSemverString() string {
	return fmt.Sprintf("v%s.%s", v.Major, v.Minor)
}

// srosVersionFromImage retrieves the SR OS version from the container image
// by inspecting the image layers without spawning a container.
func (n *sros) srosVersionFromImage(ctx context.Context) (*SrosVersion, error) {
	// Try to read from image config labels first (if set by image build)
	log.Debugf("Inspecting image %v for SR OS version retrieval", n.Cfg.Image)
	imageInspect, err := n.Runtime.InspectImage(ctx, n.Cfg.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image %s: %w", n.Cfg.Image, err)
	}
	vendor, okVendor := imageInspect.Config.Labels[srosImageVendorLabel]
	image, okTitle := imageInspect.Config.Labels[srosImageTitleLabel]
	version, okVersion := imageInspect.Config.Labels[srosImageVersionLabel]

	if okVendor && okTitle && okVersion && vendor == srosImageVendor && image == srosImageTitle {
		return n.parseVersionString(version), nil
	}

	// Fallback: read directly from image layers via graph driver
	log.Debug("Image label not found, reading version from image layers",
		"node", n.Cfg.ShortName, "image", n.Cfg.Image)

	version, err = n.readVersionFromImageLayers(ctx, imageInspect)
	if err != nil {
		log.Warn("Failed to extract SR OS version from image layers, using default",
			"node", n.Cfg.ShortName, "image", n.Cfg.Image, "error", err)
		// Return nil for version when error occurs
		return nil, err
	}

	return n.parseVersionString(version), err
}

// ReadFileFromImageInspect reads a file from the image filesystem using the graph driver's
// UpperDir or MergedDir (same approach as srosVersionFromImage), without spawning a container.
// containerPath is the path inside the image (e.g. "/opt/nokia/chassis_info.json").
// Returns the file contents or an error if the path is not available in the graph driver.
func ReadFileFromImageInspect(imageInspect *clabruntime.ImageInspect, containerPath string) ([]byte, error) {
	// Path in graph driver is relative to root (no leading slash).
	relPath := strings.TrimPrefix(filepath.Clean(containerPath), string(filepath.Separator))
	if relPath == "" {
		relPath = containerPath
	}
	// Ensure we use forward slashes for the path inside the image.
	relPath = filepath.ToSlash(relPath)

	if imageInspect.GraphDriver.Data.UpperDir != "" {
		hostPath := filepath.Join(imageInspect.GraphDriver.Data.UpperDir, relPath)
		content, err := os.ReadFile(hostPath)
		if err == nil {
			return content, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s from UpperDir: %w", containerPath, err)
		}
	}

	if imageInspect.GraphDriver.Data.MergedDir != "" {
		hostPath := filepath.Join(imageInspect.GraphDriver.Data.MergedDir, relPath)
		content, err := os.ReadFile(hostPath)
		if err == nil {
			return content, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s from MergedDir: %w", containerPath, err)
		}
	}

	return nil, fmt.Errorf(
		"%s not found in image graph driver (UpperDir/MergedDir)",
		containerPath,
	)
}

// readVersionFromImageLayers reads the SR OS version from the /etc/sros-version file
// directly from image layers using the Docker graph driver's UpperDir
// without extracting the entire image.
func (n *sros) readVersionFromImageLayers(
	_ context.Context,
	imageInspect *clabruntime.ImageInspect,
) (string, error) {
	content, err := ReadFileFromImageInspect(imageInspect, srosVersionFilePath)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(content))
	log.Debug("Found SR OS version in image layers",
		"node", n.Cfg.ShortName,
		"version", version)
	return version, nil
}
