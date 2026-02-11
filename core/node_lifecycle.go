package core

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const (
	parkingNetnsPrefix = "clab-park-"
	// Be conservative and keep the name comfortably within Linux NAME_MAX (255 bytes).
	maxParkingNetnsNameLen = 200
	vrnetlabVersionLabel   = "vrnetlab-version"
)

var (
	imageTagRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// StopNodes stops one or more deployed nodes without losing their dataplane links by parking
// the node's interfaces in a dedicated network namespace before stopping the container.
func (c *CLab) StopNodes(ctx context.Context, nodeNames []string) error {
	if len(nodeNames) == 0 {
		return fmt.Errorf("%w: at least one node name is required", claberrors.ErrIncorrectInput)
	}

	return c.withLabLock(func() error {
		if err := c.ResolveLinks(); err != nil {
			return err
		}

		nsProviders := c.namespaceShareProviders()

		for _, nodeName := range nodeNames {
			n, err := c.validateLifecycleNode(ctx, nodeName, nsProviders)
			if err != nil {
				return err
			}

			status := n.GetRuntime().GetContainerStatus(ctx, n.Config().LongName)
			switch status {
			case clabruntime.Running:
				if err := c.stopNode(ctx, n); err != nil {
					return err
				}
			case clabruntime.Stopped:
				log.Debugf("node %q already stopped, skipping", nodeName)
			default:
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}
		}

		return nil
	})
}

// StartNodes starts one or more stopped nodes and restores their parked interfaces back into the
// container network namespace.
func (c *CLab) StartNodes(ctx context.Context, nodeNames []string) error {
	if len(nodeNames) == 0 {
		return fmt.Errorf("%w: at least one node name is required", claberrors.ErrIncorrectInput)
	}

	return c.withLabLock(func() error {
		if err := c.ResolveLinks(); err != nil {
			return err
		}

		nsProviders := c.namespaceShareProviders()

		for _, nodeName := range nodeNames {
			n, err := c.validateLifecycleNode(ctx, nodeName, nsProviders)
			if err != nil {
				return err
			}

			status := n.GetRuntime().GetContainerStatus(ctx, n.Config().LongName)
			switch status {
			case clabruntime.Running:
				log.Debugf("node %q already running, skipping", nodeName)
				continue
			case clabruntime.Stopped:
				// continue below
			default:
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}

			if err := c.startNode(ctx, n); err != nil {
				return err
			}
		}

		return nil
	})
}

// RestartNodes performs stop+start for each node, restoring parked interfaces.
func (c *CLab) RestartNodes(ctx context.Context, nodeNames []string) error {
	if len(nodeNames) == 0 {
		return fmt.Errorf("%w: at least one node name is required", claberrors.ErrIncorrectInput)
	}

	return c.withLabLock(func() error {
		if err := c.ResolveLinks(); err != nil {
			return err
		}

		nsProviders := c.namespaceShareProviders()

		for _, nodeName := range nodeNames {
			n, err := c.validateLifecycleNode(ctx, nodeName, nsProviders)
			if err != nil {
				return err
			}

			status := n.GetRuntime().GetContainerStatus(ctx, n.Config().LongName)
			if status == clabruntime.NotFound {
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}

			if status == clabruntime.Running {
				if err := c.stopNode(ctx, n); err != nil {
					return err
				}
			}

			// Start/restore is only meaningful when a node is stopped and its interfaces are parked.
			if err := c.startNode(ctx, n); err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *CLab) stopNode(ctx context.Context, n clabnodes.Node) error {
	cfg := n.Config()

	// Validate link types up-front to avoid partial parking.
	for _, ep := range n.GetEndpoints() {
		if ep.GetLink().GetType() != clablinks.LinkTypeVEth {
			return fmt.Errorf(
				"node %q lifecycle supports only veth links, got %q for endpoint %s",
				cfg.ShortName,
				ep.GetLink().GetType(),
				ep.String(),
			)
		}
	}

	parkName := parkingNetnsName(cfg.LongName)
	parkPath, err := ensureNamedNetns(parkName)
	if err != nil {
		return fmt.Errorf("node %q failed creating parking netns: %w", cfg.ShortName, err)
	}

	parkNS, err := ns.GetNS(parkPath)
	if err != nil {
		return fmt.Errorf("node %q failed opening parking netns: %w", cfg.ShortName, err)
	}
	defer parkNS.Close()

	nodeNSPath, err := n.GetNSPath(ctx)
	if err != nil {
		return fmt.Errorf("node %q failed getting netns path: %w", cfg.ShortName, err)
	}

	nodeNS, err := ns.GetNS(nodeNSPath)
	if err != nil {
		return fmt.Errorf("node %q failed opening netns: %w", cfg.ShortName, err)
	}
	defer nodeNS.Close()

	// Move dataplane interfaces into the parking netns while the container netns is still alive.
	moved, err := moveEndpointsBetweenNetNS(nodeNS, parkNS, n.GetEndpoints(), func(l netlink.Link, ep clablinks.Endpoint) error {
		// Ensure peers observe link-down while the node is stopped.
		if err := netlink.LinkSetDown(l); err != nil {
			return fmt.Errorf("failed setting %s down: %w", ep.String(), err)
		}
		return nil
	})
	if err != nil {
		// Roll back any endpoints already moved to the parking namespace.
		if len(moved) > 0 {
			if _, rbErr := moveEndpointsBetweenNetNS(parkNS, nodeNS, moved, nil); rbErr != nil {
				return fmt.Errorf(
					"node %q failed parking interfaces: %w (rollback failed: %v)",
					cfg.ShortName,
					err,
					rbErr,
				)
			}
			_ = setEndpointsUp(nodeNS, moved)
		}
		return fmt.Errorf("node %q failed parking interfaces: %w", cfg.ShortName, err)
	}

	// Repoint /run/netns/<containerName> to the parking netns so that inspect/destroy keep working.
	if err := clabutils.LinkContainerNS(parkPath, cfg.LongName); err != nil {
		return fmt.Errorf("node %q failed linking parking netns: %w", cfg.ShortName, err)
	}

	c.preStopCleanup(ctx, n)

	if err := n.GetRuntime().StopContainer(ctx, cfg.LongName); err != nil {
		// Docker/podman may return an error while the container is already stopped (timeout, API hiccup).
		// Treat this as success if the desired state is reached.
		status := n.GetRuntime().GetContainerStatus(ctx, cfg.LongName)
		if status == clabruntime.Stopped {
			log.Warnf("node %q stop returned error but container is stopped: %v", cfg.ShortName, err)
			return nil
		}

		// Roll back to a running node with interfaces restored.
		if linkErr := clabutils.LinkContainerNS(nodeNSPath, cfg.LongName); linkErr != nil {
			log.Warnf("node %q failed restoring /run/netns symlink after stop error: %v", cfg.ShortName, linkErr)
		}
		if _, rbErr := moveEndpointsBetweenNetNS(parkNS, nodeNS, moved, nil); rbErr != nil {
			return fmt.Errorf(
				"node %q failed stopping container: %w (rollback failed restoring interfaces: %v)",
				cfg.ShortName,
				err,
				rbErr,
			)
		}
		_ = setEndpointsUp(nodeNS, moved)

		return fmt.Errorf("node %q failed stopping container: %w", cfg.ShortName, err)
	}

	return nil
}

func (c *CLab) startNode(ctx context.Context, n clabnodes.Node) error {
	cfg := n.Config()

	// Validate link types up-front to avoid partial restore.
	for _, ep := range n.GetEndpoints() {
		if ep.GetLink().GetType() != clablinks.LinkTypeVEth {
			return fmt.Errorf(
				"node %q lifecycle supports only veth links, got %q for endpoint %s",
				cfg.ShortName,
				ep.GetLink().GetType(),
				ep.String(),
			)
		}
	}

	parkName := parkingNetnsName(cfg.LongName)
	parkPath := filepath.Join("/run/netns", parkName)
	if !clabutils.FileOrDirExists(parkPath) {
		return fmt.Errorf(
			"node %q has no parking netns %q; seamless start requires stopping via containerlab",
			cfg.ShortName,
			parkName,
		)
	}

	parkNS, err := ns.GetNS(parkPath)
	if err != nil {
		return fmt.Errorf("node %q failed opening parking netns: %w", cfg.ShortName, err)
	}
	defer parkNS.Close()

	_, err = n.GetRuntime().StartContainer(ctx, cfg.LongName, n)
	if err != nil {
		return fmt.Errorf("node %q failed starting container: %w", cfg.ShortName, err)
	}

	nodeNSPath, err := n.GetNSPath(ctx)
	if err != nil {
		// Try to keep destroy/inspect operational by repointing back to the parking netns.
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed getting netns path: %w", cfg.ShortName, err)
	}

	nodeNS, err := ns.GetNS(nodeNSPath)
	if err != nil {
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed opening netns: %w", cfg.ShortName, err)
	}
	defer nodeNS.Close()

	// Move interfaces back into the container netns.
	moved, err := moveEndpointsBetweenNetNS(parkNS, nodeNS, n.GetEndpoints(), nil)
	if err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_, _ = moveEndpointsBetweenNetNS(nodeNS, parkNS, moved, func(l netlink.Link, ep clablinks.Endpoint) error {
			_ = netlink.LinkSetDown(l)
			return nil
		})
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed restoring interfaces: %w", cfg.ShortName, err)
	}

	// Bring restored interfaces up.
	if err := setEndpointsUp(nodeNS, moved); err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_, _ = moveEndpointsBetweenNetNS(nodeNS, parkNS, moved, func(l netlink.Link, ep clablinks.Endpoint) error {
			_ = netlink.LinkSetDown(l)
			return nil
		})
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed enabling interfaces: %w", cfg.ShortName, err)
	}

	return nil
}

func (c *CLab) validateLifecycleNode(
	ctx context.Context,
	nodeName string,
	nsProviders map[string][]string,
) (clabnodes.Node, error) {
	n, ok := c.Nodes[nodeName]
	if !ok {
		return nil, fmt.Errorf("%w: node %q is not present in the topology", claberrors.ErrIncorrectInput, nodeName)
	}

	cfg := n.Config()

	if cfg.IsRootNamespaceBased {
		return nil, fmt.Errorf("node %q is root-namespace based and cannot be stopped/started", nodeName)
	}

	if cfg.AutoRemove {
		return nil, fmt.Errorf("node %q has auto-remove enabled and is not supported", nodeName)
	}

	containers, err := n.GetContainers(ctx)
	if err != nil {
		return nil, err
	}
	if len(containers) != 1 {
		return nil, fmt.Errorf("node %q is not supported (expected 1 container, got %d)", nodeName, len(containers))
	}

	if strings.HasPrefix(cfg.NetworkMode, "container:") {
		return nil, fmt.Errorf("node %q uses network-mode %q and is not supported", nodeName, cfg.NetworkMode)
	}
	if dependers := nsProviders[nodeName]; len(dependers) > 0 {
		return nil, fmt.Errorf(
			"node %q is used as a network-mode provider for %v and is not supported",
			nodeName,
			dependers,
		)
	}

	return n, nil
}

func (c *CLab) namespaceShareProviders() map[string][]string {
	providers := make(map[string][]string)
	for _, n := range c.Nodes {
		netModeArr := strings.SplitN(n.Config().NetworkMode, ":", 2) //nolint:mnd
		if len(netModeArr) != 2 || netModeArr[0] != "container" {
			continue
		}
		ref := netModeArr[1]
		if _, exists := c.Nodes[ref]; !exists {
			continue
		}
		providers[ref] = append(providers[ref], n.Config().ShortName)
	}
	return providers
}

func (c *CLab) withLabLock(f func() error) error {
	lockPath := filepath.Join(c.TopoPaths.TopologyLabDir(), ".clab.lock")
	if c.TopoPaths.TopologyLabDir() == "" || !clabutils.DirExists(c.TopoPaths.TopologyLabDir()) {
		lockPath = c.fallbackLockPath()
	}

	if err := os.MkdirAll(filepath.Dir(lockPath), clabconstants.PermissionsDirDefault); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, clabconstants.PermissionsFileDefault)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)

	return f()
}

func (c *CLab) fallbackLockPath() string {
	baseDir := filepath.Join(c.TopoPaths.ClabTmpDir(), "locks")

	input := c.Config.Name
	if c.TopoPaths != nil && c.TopoPaths.TopologyFileIsSet() {
		input = c.TopoPaths.TopologyFilenameAbsPath() + "|" + c.Config.Name
	}

	sum := sha1.Sum([]byte(input))
	suffix := hex.EncodeToString(sum[:])[:10]
	return filepath.Join(baseDir, suffix+".lock")
}

func parkingNetnsName(containerName string) string {
	name := parkingNetnsPrefix + containerName
	if len(name) <= maxParkingNetnsNameLen {
		return name
	}

	sum := sha1.Sum([]byte(containerName))
	suffix := hex.EncodeToString(sum[:])[:10]

	// leave room for "-" + suffix
	maxBaseLen := maxParkingNetnsNameLen - 1 - len(suffix)
	if maxBaseLen < 1 {
		return suffix
	}

	return name[:maxBaseLen] + "-" + suffix
}

func ensureNamedNetns(name string) (string, error) {
	nspath := filepath.Join("/run/netns", name)
	if clabutils.FileOrDirExists(nspath) {
		return nspath, nil
	}

	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		return "", err
	}
	defer origNS.Close()
	defer func() {
		_ = netns.Set(origNS)
	}()

	newNS, err := netns.NewNamed(name)
	if err != nil {
		if os.IsExist(err) && clabutils.FileOrDirExists(nspath) {
			return nspath, nil
		}
		return "", err
	}
	newNS.Close()

	return nspath, nil
}

func findLinkForEndpoint(ep clablinks.Endpoint) (netlink.Link, error) {
	ifaceName := ep.GetIfaceName()
	if ifaceName != "" {
		if l, err := netlink.LinkByName(ifaceName); err == nil {
			return l, nil
		}
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	wantAlt := clablinks.SanitizeInterfaceName(ep.GetIfaceName())
	wantAlias := ep.GetIfaceAlias()
	wantAltAlias := ""
	if wantAlias != "" {
		wantAltAlias = clablinks.SanitizeInterfaceName(wantAlias)
	}

	for _, l := range links {
		if wantAlias != "" && l.Attrs().Alias == wantAlias {
			return l, nil
		}
		for _, an := range l.Attrs().AltNames {
			if an == wantAlt || (wantAltAlias != "" && an == wantAltAlias) {
				return l, nil
			}
		}
	}

	return nil, fmt.Errorf("interface for endpoint %s not found", ep.String())
}

func moveEndpointsBetweenNetNS(
	fromNS ns.NetNS,
	toNS ns.NetNS,
	endpoints []clablinks.Endpoint,
	preMove func(netlink.Link, clablinks.Endpoint) error,
) ([]clablinks.Endpoint, error) {
	moved := make([]clablinks.Endpoint, 0, len(endpoints))

	err := fromNS.Do(func(_ ns.NetNS) error {
		for _, ep := range endpoints {
			link, err := findLinkForEndpoint(ep)
			if err != nil {
				return err
			}
			if preMove != nil {
				if err := preMove(link, ep); err != nil {
					return err
				}
			}
			if err := netlink.LinkSetNsFd(link, int(toNS.Fd())); err != nil {
				return fmt.Errorf("failed moving %s: %w", ep.String(), err)
			}
			moved = append(moved, ep)
		}
		return nil
	})

	return moved, err
}

func setEndpointsUp(nodeNS ns.NetNS, endpoints []clablinks.Endpoint) error {
	return nodeNS.Do(func(_ ns.NetNS) error {
		for _, ep := range endpoints {
			link, err := findLinkForEndpoint(ep)
			if err != nil {
				return err
			}
			if err := netlink.LinkSetUp(link); err != nil {
				return fmt.Errorf("failed setting %s up: %w", ep.String(), err)
			}
		}
		return nil
	})
}

func (*CLab) preStopCleanup(ctx context.Context, n clabnodes.Node) {
	if isVrnetlabNode(ctx, n) {
		preStopPrepareVrnetlabQcowAlias(ctx, n)
	}

	preStopCleanupNamedNetns(ctx, n)
}

func preStopCleanupNamedNetns(ctx context.Context, n clabnodes.Node) {
	// Best-effort cleanup for containers that create named network namespaces under /run/netns.
	// We lazily unmount active nsfs mounts and remove stale entries to avoid namespace artifacts
	// breaking subsequent starts.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := `if [ -d /run/netns ]; then ` +
		`awk '$5 ~ "^/run/netns/" {print $5}' /proc/self/mountinfo 2>/dev/null | ` +
		`while IFS= read -r mp; do ` +
		`umount -l "$mp" 2>/dev/null || true; ` +
		`rm -f "$mp" 2>/dev/null || true; ` +
		`done; ` +
		`for f in /run/netns/*; do [ -e "$f" ] || break; rm -f "$f" 2>/dev/null || true; done; ` +
		`fi`

	execCmd := clabexec.NewExecCmdFromSlice([]string{"sh", "-lc", cmd})
	if res, err := n.RunExec(ctx, execCmd); err != nil {
		log.Debugf(
			"node %q generic pre-stop named-netns cleanup skipped/failed: %v",
			n.Config().ShortName,
			err,
		)
	} else if res != nil && res.ReturnCode != 0 {
		log.Debugf(
			"node %q generic pre-stop named-netns cleanup returned code %d (stderr: %s)",
			n.Config().ShortName,
			res.ReturnCode,
			res.Stderr,
		)
	}
}

func isVrnetlabNode(ctx context.Context, n clabnodes.Node) bool {
	containers, err := n.GetContainers(ctx)
	if err != nil {
		log.Debugf(
			"node %q vrnetlab detection skipped: failed to get container metadata: %v",
			n.Config().ShortName,
			err,
		)
		return false
	}

	for _, container := range containers {
		if _, ok := container.Labels[vrnetlabVersionLabel]; ok {
			return true
		}
	}

	return false
}

func preStopPrepareVrnetlabQcowAlias(ctx context.Context, n clabnodes.Node) {
	aliasName, ok := vrnetlabQcowAliasName(n.Config().Image)
	if !ok {
		log.Debugf(
			"node %q pre-stop vrnetlab qcow alias skipped: unable to infer tag from image %q",
			n.Config().ShortName,
			n.Config().Image,
		)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Some vrnetlab nodes rename the original versioned qcow image after first boot and fail on
	// subsequent starts when they try to rediscover a versioned qcow filename. If there is exactly
	// one non-overlay qcow file in / and our alias is absent, create a hardlink alias based on the
	// image tag.
	cmd := fmt.Sprintf(
		`alias="/%s"; `+
			`[ -e "$alias" ] && exit 0; `+
			`src=""; `+
			`if [ -f /sros.qcow2 ] && [ "/sros.qcow2" != "$alias" ]; then `+
			`src="/sros.qcow2"; `+
			`else `+
			`set -- /*.qcow2; `+
			`if [ "$1" != "/*.qcow2" ]; then `+
			`for f in "$@"; do `+
			`[ "$f" = "$alias" ] && continue; `+
			`base="${f##*/}"; `+
			`case "$base" in *overlay*.qcow2) continue ;; esac; `+
			`if [ -n "$src" ]; then src=""; break; fi; `+
			`src="$f"; `+
			`done; `+
			`fi; `+
			`fi; `+
			`[ -n "$src" ] || exit 0; `+
			`ln "$src" "$alias"`,
		aliasName,
	)

	execCmd := clabexec.NewExecCmdFromSlice([]string{"sh", "-lc", cmd})
	res, err := n.RunExec(ctx, execCmd)
	if err != nil {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias preparation failed: %v",
			n.Config().ShortName,
			err,
		)
		return
	}

	if res != nil && res.ReturnCode != 0 {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias prep returned code %d (stderr: %s)",
			n.Config().ShortName,
			res.ReturnCode,
			res.Stderr,
		)
	}
}

func vrnetlabQcowAliasName(image string) (string, bool) {
	tag, ok := imageTag(image)
	if !ok {
		return "", false
	}

	return "clab-" + tag + ".qcow2", true
}

func imageTag(image string) (string, bool) {
	if at := strings.LastIndex(image, "@"); at != -1 {
		image = image[:at]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 || lastColon < lastSlash {
		return "", false
	}

	tag := image[lastColon+1:]
	if tag == "" {
		return "", false
	}

	if !imageTagRE.MatchString(tag) {
		return "", false
	}

	return tag, true
}
