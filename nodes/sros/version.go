package sros

import (
	"context"
	_ "embed"
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

const srosVersionFilePath = "/etc/sros-version"

var (
	//go:embed configs/10_snmpv2.cfg
	snmpv2Config string

	//go:embed configs/11_logging.cfg
	loggingConfig string

	//go:embed configs/12_grpc.cfg
	grpcConfig string

	//go:embed configs/12_grpc_insecure.cfg
	grpcConfigInsecure string

	//go:embed configs/ixr/12_grpc.cfg
	grpcConfigIXR string

	//go:embed configs/ixr/12_grpc_insecure.cfg
	grpcConfigIXRInsecure string

	//go:embed configs/sar/12_grpc.cfg
	grpcConfigSAR string

	//go:embed configs/sar/12_grpc_insecure.cfg
	grpcConfigSARInsecure string

	//go:embed configs/13_netconf.cfg
	netconfConfig string

	//go:embed configs/14_system.cfg
	systemCfg string

	//go:embed configs/ixr/14_system.cfg
	systemCfgIXR string

	//go:embed configs/sar/14_system.cfg
	systemCfgSAR string

	//go:embed configs/15_ssh.cfg
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

// srosVersionFromImage retrieves the SR OS version from the container image
// by inspecting the image layers without spawning a container.
func (n *sros) srosVersionFromImage(ctx context.Context) (*SrosVersion, error) {
	// Try to read from image config labels first (if set by image build)
	log.Debugf("Inspecting image %v for SR OS version retrieval", n.Cfg.Image)
	imageInspect, err := n.Runtime.InspectImage(ctx, n.Cfg.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image %s: %w", n.Cfg.Image, err)
	}

	if version, exists := imageInspect.Config.Labels["sros.version"]; exists {
		return n.parseVersionString(version), err
	}

	// Fallback: read directly from image layers via graph driver
	log.Debug("Image label not found, reading version from image layers",
		"node", n.Cfg.ShortName, "image", n.Cfg.Image)

	version, err := n.readVersionFromImageLayers(ctx, imageInspect)
	if err != nil {
		log.Warn("Failed to extract SR OS version from image layers, using default",
			"node", n.Cfg.ShortName, "image", n.Cfg.Image, "error", err)
		// Return nil for version when error occurs
		return nil, err
	}

	return n.parseVersionString(version), err
}

// readVersionFromImageLayers reads the SR OS version from the /etc/sros-version file
// directly from image layers using the Docker graph driver's UpperDir
// without extracting the entire image.
func (n *sros) readVersionFromImageLayers(
	_ context.Context,
	imageInspect *clabruntime.ImageInspect,
) (string, error) {
	// First, try to use the GraphDriver.Data.UpperDir if available
	if imageInspect.GraphDriver.Data.UpperDir != "" {
		versionPath := filepath.Join(imageInspect.GraphDriver.Data.UpperDir, srosVersionFilePath)

		log.Debug("Attempting to read SR OS version from UpperDir",
			"node", n.Cfg.ShortName,
			"path", versionPath)

		content, err := os.ReadFile(versionPath)
		if err == nil {
			version := strings.TrimSpace(string(content))
			log.Debug("Found SR OS version in UpperDir",
				"node", n.Cfg.ShortName,
				"version", version)
			return version, nil
		}

		// Log the error and only fallback if it's a "file not found" error
		if !errors.Is(err, os.ErrNotExist) {
			log.Warn("Failed to read SR OS version from UpperDir",
				"node", n.Cfg.ShortName,
				"path", versionPath,
				"error", err)
			return "", fmt.Errorf("failed to read SR OS version from UpperDir: %w", err)
		}

		log.Debug("sros-version file not found in UpperDir, trying MergedDir",
			"node", n.Cfg.ShortName)
	}

	// Fallback: try MergedDir if available
	if imageInspect.GraphDriver.Data.MergedDir != "" {
		versionPath := filepath.Join(imageInspect.GraphDriver.Data.MergedDir, srosVersionFilePath)

		log.Debug("Attempting to read SR OS version from MergedDir",
			"node", n.Cfg.ShortName,
			"path", versionPath)

		content, err := os.ReadFile(versionPath)
		if err == nil {
			version := strings.TrimSpace(string(content))
			log.Debug("Found SR OS version in MergedDir",
				"node", n.Cfg.ShortName,
				"version", version)
			return version, nil
		}

		// Log the specific error
		if !errors.Is(err, os.ErrNotExist) {
			log.Warn("Failed to read SR OS version from MergedDir",
				"node", n.Cfg.ShortName,
				"path", versionPath,
				"error", err)
			return "", fmt.Errorf("failed to read SR OS version from MergedDir: %w", err)
		}

		log.Debug("SR OS version file not found in MergedDir",
			"path",
			srosVersionFilePath,
			"node", n.Cfg.ShortName)
	}

	return "", fmt.Errorf("%s file not found in image graph driver directories or layers", srosVersionFilePath)
}
