package sros

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	"github.com/srl-labs/containerlab/runtime"
)

var (
	//go:embed configs/snmpv2.cfg
	snmpv2Config string

	//go:embed configs/grpc.cfg
	grpcConfig string

	//go:embed configs/grpc_insecure.cfg
	grpcConfigInsecure string

	//go:embed configs/grpc_ixr.cfg
	grpcConfigIXR string

	//go:embed configs/grpc_ixr_insecure.cfg
	grpcConfigIXRInsecure string

	//go:embed configs/netconf.cfg
	netconfConfig string

	//go:embed configs/logging.cfg
	loggingConfig string

	//go:embed configs/system.cfg
	systemCfg string

	//go:embed configs/system_ixr.cfg
	systemCfgIXR string

	//go:embed configs/ssh.cfg
	sshConfig string
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

// getSrosVersionFromImage retrieves the SR OS version from the container image
// by inspecting the image layers without spawning a container.
func (n *sros) getSrosVersionFromImage(ctx context.Context) (*SrosVersion, error) {
	// Try to read from image config labels first (if set by image build)
	log.Debugf("Inspecting image %v for SR OS version retrieval", n.Cfg.Image)
	imageInspect, err := n.Runtime.InspectImage(ctx, n.Cfg.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image %s: %w", n.Cfg.Image, err)
	}

	if version, exists := imageInspect.Config.Labels["sros.version"]; exists {
		return n.parseVersionString(version), nil
	}

	// Fallback: read directly from image layers via graph driver
	log.Debug("Image label not found, reading version from image layers",
		"node", n.Cfg.ShortName, "image", n.Cfg.Image)

	version, err := n.readVersionFromImageLayers(ctx, imageInspect)
	if err != nil {
		log.Warn("Failed to extract SR OS version from image layers, using default",
			"node", n.Cfg.ShortName, "error", err)
		// Return nil for version when error occurs
		return nil, err
	}

	return n.parseVersionString(version), nil
}

// readVersionFromImageLayers reads the sros-version file directly from image layers
// using the Docker graph driver's UpperDir without extracting the entire image.
func (n *sros) readVersionFromImageLayers(
	_ context.Context,
	imageInspect *runtime.ImageInspect,
) (string, error) {
	// First, try to use the GraphDriver.Data.UpperDir if available
	if imageInspect.GraphDriver.Data.UpperDir != "" {
		versionPath := filepath.Join(imageInspect.GraphDriver.Data.UpperDir, "etc", "sros-version")

		log.Debug("Attempting to read sros-version from UpperDir",
			"node", n.Cfg.ShortName,
			"path", versionPath)

		if content, err := os.ReadFile(versionPath); err == nil {
			version := strings.TrimSpace(string(content))
			log.Debug("Found SR OS version in UpperDir",
				"node", n.Cfg.ShortName,
				"version", version)
			return version, nil
		}
	}

	// Fallback: try MergedDir if available
	if imageInspect.GraphDriver.Data.MergedDir != "" {
		versionPath := filepath.Join(imageInspect.GraphDriver.Data.MergedDir, "etc", "sros-version")

		log.Debug("Attempting to read sros-version from MergedDir",
			"node", n.Cfg.ShortName,
			"path", versionPath)

		if content, err := os.ReadFile(versionPath); err == nil {
			version := strings.TrimSpace(string(content))
			log.Debug("Found SR OS version in MergedDir",
				"node", n.Cfg.ShortName,
				"version", version)
			return version, nil
		}
	}

	return "", fmt.Errorf("sros-version file not found in image graph driver directories or layers")
}
