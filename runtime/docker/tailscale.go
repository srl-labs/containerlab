package docker

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

//go:embed scripts/nat-setup.sh
var natSetupScript string

//go:embed scripts/dns-proxy.py
var dnsProxyScript string

//go:embed scripts/coredns-install.sh
var coreDNSInstallScript string

//go:embed scripts/Corefile.tmpl
var corefileTemplate string

const (
	// Default Tailscale image if not specified in config
	defaultTailscaleImage  = "tailscale/tailscale:latest"
	// Default CoreDNS version if not specified in config
	defaultCoreDNSVersion  = "1.13.1"
)

// LabContext provides context needed for Tailscale deployment.
type LabContext struct {
	Name     string
	Prefix   string
	Owner    string
	LabDir   string
	TopoFile string
}

// DeployTailscale deploys a Tailscale container for VPN access to the management network.
// It accepts the lab context to access lab metadata without storing it in DockerRuntime.
func (d *DockerRuntime) DeployTailscale(ctx context.Context, labCtx *LabContext) error {
	
	// If Tailscale config is not defined at all, skip
	if d.mgmt.Tailscale == nil {
		log.Debug("Tailscale is not configured, skipping deployment")
		return nil
	}

	// If Tailscale section exists but enabled is explicitly set to false, skip
	// Otherwise deploy (enabled=true by default when section exists and enabled is not set or set to true)
	if d.mgmt.Tailscale.Enabled != nil && *d.mgmt.Tailscale.Enabled == false {
		log.Debug("Tailscale is explicitly disabled, skipping deployment")
		return nil
	}

	log.Info("Deploying Tailscale VPN container for management network")

	// Validate configuration
	if err := d.validateTailscaleConfig(); err != nil {
		return fmt.Errorf("invalid Tailscale configuration: %w", err)
	}

	// Build container name following containerlab naming convention: <prefix>-<lab-name>-<node-name>
	prefix := labCtx.Prefix
	if prefix == "" {
		prefix = "clab"
	}
	name := labCtx.Name
	if name == "" {
		name = "default"
	}
	
	containerName := fmt.Sprintf("%s-%s-tailscale", prefix, name)
	hostname := containerName // Use the full container name as hostname to match containerlab convention

	// Determine which Tailscale image to use
	tailscaleImage := d.mgmt.Tailscale.Image
	if tailscaleImage == "" {
		tailscaleImage = defaultTailscaleImage
	}

	// Check if container already exists
	existingContainers, err := d.Client.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range existingContainers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				log.Infof("Tailscale container %s already exists, skipping creation", containerName)
				
				// Start the container if it's not running
				if c.State != "running" {
					log.Infof("Starting existing Tailscale container %s", containerName)
					if err := d.Client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
						return fmt.Errorf("failed to start Tailscale container: %w", err)
					}
				}
				return nil
			}
		}
	}

	// Pull Tailscale image
	log.Infof("Pulling Tailscale image: %s", tailscaleImage)
	if err := d.PullImage(ctx, tailscaleImage, clabtypes.PullPolicyIfNotPresent); err != nil {
		return fmt.Errorf("failed to pull Tailscale image: %w", err)
	}

	// Prepare environment variables
	env := d.prepareTailscaleEnv(labCtx)

	// Get IPv4 address for the container
	ipv4Address := d.mgmt.Tailscale.IPv4Address
	if ipv4Address == "" {
		// Default to last IP in the mgmt subnet
		ipv4Address, err = d.getLastIPFromSubnet(d.mgmt.IPv4Subnet)
		if err != nil {
			return fmt.Errorf("failed to determine Tailscale container IPv4: %w", err)
		}
	}

	// Get IPv6 address for the container
	var ipv6Address string
	if d.mgmt.Tailscale.IPv6Address != "" {
		// Use explicitly configured IPv6 address
		ipv6Address = d.mgmt.Tailscale.IPv6Address
	} else if d.mgmt.IPv6Subnet != "" {
		// Default to last IP in the mgmt IPv6 subnet
		ipv6Address, err = d.getLastIPFromSubnet(d.mgmt.IPv6Subnet)
		if err != nil {
			return fmt.Errorf("failed to determine Tailscale container IPv6: %w", err)
		}
	}

	// Prepare custom startup command if NAT is configured
	// If not configured, cmd will be nil and Docker will use the image's default CMD
	var cmd []string
	if d.mgmt.Tailscale.OneToOneNAT != "" {
		cmd = d.prepareStartupCmdWithNAT()
	}

	// Create container configuration
	containerConfig := &container.Config{
		Image:    tailscaleImage,
		Hostname: hostname,
		Env:      env,
		Cmd:      cmd,
		Labels: map[string]string{
			clabconstants.Containerlab:      name,
			clabconstants.NodeName:          "tailscale",
			clabconstants.LongName:          containerName,
			clabconstants.NodeGroup:         "",
			clabconstants.NodeKind:          "tailscale",
			clabconstants.Owner:             labCtx.Owner,
			clabconstants.NodeMgmtNetBr:     d.mgmt.Bridge,
			clabconstants.NodeLabDir:        labCtx.LabDir,
			clabconstants.TopoFile:          labCtx.TopoFile,
			clabconstants.IsInfrastructure:  "true",
			"containerlab.tailscale":        "true",
			"containerlab.mgmt-network":     d.mgmt.Network,
		},
		Healthcheck: &container.HealthConfig{
			Test:        []string{"CMD-SHELL", "tailscale status --json | grep -q '\"BackendState\": \"Running\"'"},
			Interval:    30000000000,  // 30 seconds
			Timeout:     10000000000,  // 10 seconds
			StartPeriod: 60000000000,  // 60 seconds
			Retries:     3,
		},
	}

	hostConfig := &container.HostConfig{
		CapAdd: []string{"NET_ADMIN", "NET_RAW"},
		NetworkMode: container.NetworkMode(d.mgmt.Network),
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Sysctls: map[string]string{
			"net.ipv4.ip_forward":      "1",
			"net.ipv6.conf.all.forwarding": "1",
		},
	}

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			d.mgmt.Network: {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: ipv4Address,
					IPv6Address: ipv6Address,
				},
			},
		},
	}

	// Create the container
	logMsg := fmt.Sprintf("Creating Tailscale container %s with IPv4 %s", containerName, ipv4Address)
	if ipv6Address != "" {
		logMsg += fmt.Sprintf(" and IPv6 %s", ipv6Address)
	}
	log.Info(logMsg)
	resp, err := d.Client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create Tailscale container: %w", err)
	}

	// Start the container
	log.Infof("Starting Tailscale container %s", containerName)
	if err := d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start Tailscale container: %w", err)
	}

	log.Info("Tailscale VPN container deployed successfully")

	// Install and configure CoreDNS if DNS is enabled
	if d.mgmt.Tailscale.DNS != nil && d.mgmt.Tailscale.DNS.Enabled != nil && *d.mgmt.Tailscale.DNS.Enabled {
		// Wait for Tailscale to be ready before setting up DNS
		if err := d.waitForTailscaleReady(ctx, resp.ID, 30*time.Second); err != nil {
			log.Warnf("Tailscale readiness check failed: %v, attempting DNS setup anyway", err)
		}
		if err := d.setupTailscaleDNS(ctx, resp.ID, labCtx); err != nil {
			return fmt.Errorf("failed to setup DNS in Tailscale container: %w", err)
		}
	}

	return nil
}

// setupTailscaleDNS installs and configures CoreDNS in the Tailscale container.
func (d *DockerRuntime) setupTailscaleDNS(ctx context.Context, containerID string, labCtx *LabContext) error {
	log.Info("Setting up CoreDNS in Tailscale container")

	// Determine CoreDNS version to use
	coreDNSVersion := defaultCoreDNSVersion
	if d.mgmt.Tailscale.DNS != nil && d.mgmt.Tailscale.DNS.CoreDNSVersion != "" {
		coreDNSVersion = d.mgmt.Tailscale.DNS.CoreDNSVersion
	}

	log.Infof("Installing CoreDNS version %s", coreDNSVersion)

	// Determine if we need Python (only when NAT + DNS are both enabled)
	needsPython := d.mgmt.Tailscale.OneToOneNAT != ""

	// Generate installation script from template
	installData := struct {
		CoreDNSVersion string
		NeedsPython    string
	}{
		CoreDNSVersion: coreDNSVersion,
		NeedsPython:    fmt.Sprintf("%t", needsPython),
	}

	tmpl, err := template.New("coredns-install").Parse(coreDNSInstallScript)
	if err != nil {
		return fmt.Errorf("failed to parse CoreDNS install template: %w", err)
	}

	var installBuf bytes.Buffer
	if err := tmpl.Execute(&installBuf, installData); err != nil {
		return fmt.Errorf("failed to execute CoreDNS install template: %w", err)
	}

	// Execute the installation script
	log.Debug("Executing CoreDNS installation script")
	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", installBuf.String()},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := d.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec for CoreDNS installation: %w", err)
	}

	attachResp, err := d.Client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to CoreDNS installation: %w", err)
	}
	defer attachResp.Close()

	// Read output in background (this waits for command to complete)
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		_, err := stdcopy.StdCopy(&outBuf, &errBuf, attachResp.Reader)
		outputDone <- err
	}()

	// Wait for command to complete
	select {
	case err := <-outputDone:
		if err != nil {
			log.Warnf("Error reading installation output: %v", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// Check if installation was successful
	inspectResp, err := d.Client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect CoreDNS installation: %w", err)
	}
	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("CoreDNS installation failed with exit code %d: %s", inspectResp.ExitCode, errBuf.String())
	}

	log.Info("CoreDNS installation completed successfully")

	// Create initial empty Corefile
	initialCorefile := d.generateCorefile(labCtx.Name, []DNSRecord{})
	if err := d.writeFileToContainer(ctx, containerID, "/etc/coredns/Corefile", initialCorefile); err != nil {
		return fmt.Errorf("failed to write initial Corefile: %w", err)
	}

	// Create empty hosts file
	initialHosts := d.generateHostsFile(labCtx.Name, []DNSRecord{})
	if err := d.writeFileToContainer(ctx, containerID, "/etc/coredns/hosts", initialHosts); err != nil {
		return fmt.Errorf("failed to write initial hosts file: %w", err)
	}

	// Determine ports and setup DNS proxy if NAT is enabled
	coreDNSPort := 5353 // Internal port when using DNS proxy
	publicDNSPort := 53
	if d.mgmt.Tailscale.DNS != nil && d.mgmt.Tailscale.DNS.Port > 0 {
		publicDNSPort = d.mgmt.Tailscale.DNS.Port
	}
	
	useProxy := d.mgmt.Tailscale.OneToOneNAT != ""

	// If NAT is enabled, create and start DNS proxy
	if useProxy {
		log.Info("NAT is enabled, setting up DNS proxy for IP address rewriting")
		
		// Create the DNS proxy script
		proxyScript := d.generateDNSProxyScript(publicDNSPort, coreDNSPort)
		if err := d.writeFileToContainer(ctx, containerID, "/usr/local/bin/dns-proxy.py", proxyScript); err != nil {
			return fmt.Errorf("failed to write DNS proxy script: %w", err)
		}
	} else {
		// No NAT, CoreDNS listens on public port directly
		coreDNSPort = publicDNSPort
	}

	// Start CoreDNS in background on internal or public port
	// redirecting output to PID 1's stdout for docker logs visibility
	// Use awk to prepend timestamp and "coredns:" prefix
	startCmd := fmt.Sprintf("nohup sh -c '/usr/local/bin/coredns -conf /etc/coredns/Corefile -dns.port=%d 2>&1 | awk \"{print strftime(\\\"%%Y/%%m/%%d %%H:%%M:%%S\\\"), \\\"coredns:\\\", \\$0; fflush()}\"' >/proc/1/fd/1 2>&1 </dev/null &", coreDNSPort)
	execConfig = container.ExecOptions{
		Cmd:          []string{"sh", "-c", startCmd},
		AttachStdout: false,
		AttachStderr: false,
	}

	execID, err = d.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec for CoreDNS start: %w", err)
	}

	if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start CoreDNS: %w", err)
	}

	// Wait for CoreDNS to start (check if process is running)
	if err := d.waitForCoreDNSReady(ctx, containerID, 10*time.Second); err != nil {
		log.Warnf("CoreDNS readiness check failed: %v, but continuing", err)
	}

	log.Infof("CoreDNS started successfully on port %d", coreDNSPort)
	
	// Start DNS proxy if NAT is enabled
	if useProxy {
		startProxyCmd := "nohup sh -c 'python3 /usr/local/bin/dns-proxy.py 2>&1 | awk \"{print strftime(\\\"%Y/%m/%d %H:%M:%S\\\"), \\\"dns-proxy:\\\", \\$0; fflush()}\"' >/proc/1/fd/1 2>&1 </dev/null &"
		execConfig := container.ExecOptions{
			Cmd:          []string{"sh", "-c", startProxyCmd},
			AttachStdout: false,
			AttachStderr: false,
		}

		execID, err := d.Client.ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			return fmt.Errorf("failed to create exec for DNS proxy start: %w", err)
		}

		if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
			return fmt.Errorf("failed to start DNS proxy: %w", err)
		}
		
		log.Infof("DNS proxy started successfully on port %d (forwarding to CoreDNS on port %d)", publicDNSPort, coreDNSPort)
	}

	return nil
}

// DestroyTailscale removes the Tailscale VPN container.
// It finds the container by labels rather than constructing the name,
// making it more robust and not requiring any lab context.
func (d *DockerRuntime) DestroyTailscale(ctx context.Context, labName string) error {
	// Build filters to find Tailscale infrastructure container by labels
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", clabconstants.Containerlab, labName))
	filter.Add("label", fmt.Sprintf("%s=%s", clabconstants.NodeKind, "tailscale"))
	filter.Add("label", fmt.Sprintf("%s=%s", clabconstants.IsInfrastructure, "true"))

	containers, err := d.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		log.Debugf("Tailscale container for lab %s not found, nothing to destroy", labName)
		return nil
	}

	// Should only be one Tailscale container per lab
	containerID := containers[0].ID
	containerName := strings.TrimPrefix(containers[0].Names[0], "/")

	log.Infof("Destroying Tailscale container %s", containerName)

	// Stop the container with a short timeout
	log.Debugf("Stopping Tailscale container %s", containerName)
	timeout := 3
	if err := d.Client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		if !dockerC.IsErrNotFound(err) {
			log.Warnf("Failed to stop Tailscale container: %v", err)
		}
	}

	// Remove the container
	log.Debugf("Removing Tailscale container %s", containerName)
	if err := d.Client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		if !dockerC.IsErrNotFound(err) {
			return fmt.Errorf("failed to remove Tailscale container: %w", err)
		}
	}

	log.Info("Tailscale container destroyed successfully")
	return nil
}

// validateTailscaleConfig validates the Tailscale configuration.
func (d *DockerRuntime) validateTailscaleConfig() error {
	if d.mgmt.Tailscale.AuthKey == "" {
		return fmt.Errorf("Tailscale authkey is required")
	}

	if d.mgmt.Tailscale.IPv4Address != "" {
		if ip := net.ParseIP(d.mgmt.Tailscale.IPv4Address); ip == nil {
			return fmt.Errorf("invalid IPv4 address: %s", d.mgmt.Tailscale.IPv4Address)
		}
	}
	
	if d.mgmt.Tailscale.IPv6Address != "" {
		if ip := net.ParseIP(d.mgmt.Tailscale.IPv6Address); ip == nil {
			return fmt.Errorf("invalid IPv6 address: %s", d.mgmt.Tailscale.IPv6Address)
		}
	}

	if d.mgmt.Tailscale.OneToOneNAT != "" {
		_, _, err := net.ParseCIDR(d.mgmt.Tailscale.OneToOneNAT)
		if err != nil {
			return fmt.Errorf("invalid one-to-one-nat subnet: %w", err)
		}
	}

	return nil
}

// prepareTailscaleEnv prepares environment variables for the Tailscale container.
func (d *DockerRuntime) prepareTailscaleEnv(labCtx *LabContext) []string {
	// Build hostname using the same convention as container name
	prefix := labCtx.Prefix
	if prefix == "" {
		prefix = "clab"
	}
	name := labCtx.Name
	if name == "" {
		name = "default"
	}
	hostname := fmt.Sprintf("%s-%s-tailscale", prefix, name)

	env := []string{
		fmt.Sprintf("TS_AUTHKEY=%s", d.mgmt.Tailscale.AuthKey),
		"TS_USERSPACE=false",
		fmt.Sprintf("TS_HOSTNAME=%s", hostname),
	}

	// Configure state directory - use ephemeral in-memory state if requested
	if d.mgmt.Tailscale.EphemeralState != nil && *d.mgmt.Tailscale.EphemeralState {
		env = append(env, "TS_STATE_DIR=mem:")
	} else {
		env = append(env, "TS_STATE_DIR=/var/lib/tailscale")
	}

	var extraArgs []string

	// Determine which routes to advertise
	var routesToAdvertise []string
	
	// If 1:1 NAT is configured, advertise the NAT subnet instead of the IPv4 mgmt subnet
	if d.mgmt.Tailscale.OneToOneNAT != "" {
		routesToAdvertise = append(routesToAdvertise, d.mgmt.Tailscale.OneToOneNAT)
	} else {
		// Advertise the management IPv4 subnet (including auto/default subnets)
		if d.mgmt.IPv4Subnet != "" {
			routesToAdvertise = append(routesToAdvertise, d.mgmt.IPv4Subnet)
		}
	}
	// Advertise the management IPv6 subnet (including auto/default subnets)
	if d.mgmt.IPv6Subnet != "" {
		routesToAdvertise = append(routesToAdvertise, d.mgmt.IPv6Subnet)
	}
	// Build TS_EXTRA_ARGS with route advertisement
	if len(routesToAdvertise) > 0 {
		routes := strings.Join(routesToAdvertise, ",")
		extraArgs = append(extraArgs, fmt.Sprintf("--advertise-routes=%s", routes))
	}

	// Add tags if specified (tags must be defined in Tailscale ACL)
	if len(d.mgmt.Tailscale.Tags) > 0 {
		// Prefix each tag with "tag:" if not already present
		taggedTags := make([]string, len(d.mgmt.Tailscale.Tags))
		for i, tag := range d.mgmt.Tailscale.Tags {
			if strings.HasPrefix(tag, "tag:") {
				taggedTags[i] = tag
			} else {
				taggedTags[i] = "tag:" + tag
			}
		}
		tagStr := strings.Join(taggedTags, ",")
		extraArgs = append(extraArgs, fmt.Sprintf("--advertise-tags=%s", tagStr))
	}

	// SNAT - Tailscale enables SNAT by default for advertised routes
	if d.mgmt.Tailscale.SNAT != nil && *d.mgmt.Tailscale.SNAT == false {
		extraArgs = append(extraArgs, "--snat-subnet-routes=false")
	}

	// Accept Routes from Tailnet - default is false
	if d.mgmt.Tailscale.AcceptRoutes != nil && *d.mgmt.Tailscale.AcceptRoutes == true {
		extraArgs = append(extraArgs, "--accept-routes")
	} else {
		extraArgs = append(extraArgs, "--accept-routes=false")
	}

	// Accept DNS from Tailnet - default is false
	if d.mgmt.Tailscale.AcceptDNS != nil && *d.mgmt.Tailscale.AcceptDNS == true {
		extraArgs = append(extraArgs, "--accept-dns")
	} else {
		extraArgs = append(extraArgs, "--accept-dns=false")
	}

	// Combine extra args if any
	if len(extraArgs) > 0 {
		env = append(env, fmt.Sprintf("TS_EXTRA_ARGS=%s", strings.Join(extraArgs, " ")))
	}

	return env
}

// getLastIPFromSubnet returns the last usable IP address in a subnet.
func (d *DockerRuntime) getLastIPFromSubnet(subnet string) (string, error) {
	if subnet == "" || subnet == "auto" {
		return "", fmt.Errorf("subnet is not specified")
	}

	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", err
	}

	lastHostIP := clabutils.LastHostIPInSubnet(ipnet)
	if lastHostIP == nil {
		return "", fmt.Errorf("could not determine last host IP in subnet %s", subnet)
	}

	return lastHostIP.String(), nil
}

// prepareStartupCmdWithNAT prepares a startup command that injects iptables NAT rules
// after Tailscale initializes. This ensures NAT rules persist across container restarts.
func (d *DockerRuntime) prepareStartupCmdWithNAT() []string {
	// Parse subnets for template
	_, mgmtNet, err := net.ParseCIDR(d.mgmt.IPv4Subnet)
	if err != nil {
		log.Warnf("Failed to parse mgmt subnet for NAT startup script: %v", err)
		return nil
	}

	_, natNet, err := net.ParseCIDR(d.mgmt.Tailscale.OneToOneNAT)
	if err != nil {
		log.Warnf("Failed to parse NAT subnet for NAT startup script: %v", err)
		return nil
	}

	// Prepare template data
	data := struct {
		MgmtSubnet string
		NatSubnet  string
	}{
		MgmtSubnet: mgmtNet.String(),
		NatSubnet:  natNet.String(),
	}

	// Execute template
	tmpl, err := template.New("nat-setup").Parse(natSetupScript)
	if err != nil {
		log.Warnf("Failed to parse NAT setup template: %v", err)
		return nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Warnf("Failed to execute NAT setup template: %v", err)
		return nil
	}

	// Return command that executes the rendered script
	return []string{"sh", "-c", buf.String()}
}

// generateDNSProxyScript creates a Python DNS proxy script that rewrites IP addresses
// in DNS responses based on whether the query came from Tailscale or local network.
func (d *DockerRuntime) generateDNSProxyScript(listenPort, backendPort int) string {
	// Parse subnets for validation
	_, _, err := net.ParseCIDR(d.mgmt.IPv4Subnet)
	if err != nil {
		log.Warnf("Failed to parse mgmt subnet for DNS proxy: %v", err)
		return ""
	}

	_, _, err = net.ParseCIDR(d.mgmt.Tailscale.OneToOneNAT)
	if err != nil {
		log.Warnf("Failed to parse NAT subnet for DNS proxy: %v", err)
		return ""
	}

	// Prepare template data
	data := struct {
		ListenPort int
		BackendPort int
		MgmtSubnet string
		NatSubnet  string
	}{
		ListenPort:  listenPort,
		BackendPort: backendPort,
		MgmtSubnet:  d.mgmt.IPv4Subnet,
		NatSubnet:   d.mgmt.Tailscale.OneToOneNAT,
	}

	// Execute template
	tmpl, err := template.New("dns-proxy").Parse(dnsProxyScript)
	if err != nil {
		log.Warnf("Failed to parse DNS proxy template: %v", err)
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Warnf("Failed to execute DNS proxy template: %v", err)
		return ""
	}

	return buf.String()
}

// UpdateTailscaleDNS updates the DNS records in the Tailscale container if DNS is enabled.
// This should be called after nodes are deployed to populate DNS with node records.
func (d *DockerRuntime) UpdateTailscaleDNS(ctx context.Context, labName string, nodes map[string]interface{}) error {
	// Check if Tailscale is configured and DNS is enabled
	if d.mgmt.Tailscale == nil {
		return nil
	}

	if d.mgmt.Tailscale.DNS == nil || d.mgmt.Tailscale.DNS.Enabled == nil || !*d.mgmt.Tailscale.DNS.Enabled {
		log.Debug("Tailscale DNS is not enabled, skipping DNS record updates")
		return nil
	}

	// Find the Tailscale container
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", clabconstants.Containerlab, labName))
	filter.Add("label", fmt.Sprintf("%s=%s", clabconstants.NodeKind, "tailscale"))
	filter.Add("label", fmt.Sprintf("%s=%s", clabconstants.IsInfrastructure, "true"))

	containers, err := d.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		log.Debug("Tailscale container not found, skipping DNS updates")
		return nil
	}

	containerID := containers[0].ID

	// Generate DNS records from nodes
	dnsRecords := d.generateDNSRecords(labName, nodes)
	if len(dnsRecords) == 0 {
		log.Debug("No DNS records to update")
		return nil
	}

	// Generate CoreDNS configuration
	corefile := d.generateCorefile(labName, dnsRecords)

	// Write Corefile to container
	if err := d.writeFileToContainer(ctx, containerID, "/etc/coredns/Corefile", corefile); err != nil {
		return fmt.Errorf("failed to write Corefile: %w", err)
	}

	// Write hosts file to container
	hostsContent := d.generateHostsFile(labName, dnsRecords)
	if err := d.writeFileToContainer(ctx, containerID, "/etc/coredns/hosts", hostsContent); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	// TODO: PTR (reverse DNS) support removed temporarily
	// CoreDNS hosts plugin can generate PTR records automatically,
	// but configuration needs to be fixed to make CoreDNS authoritative
	// for reverse zones. See TODO in generateHostsFile() for details.

	// Reload CoreDNS (send SIGUSR1 to coredns process)
	if err := d.reloadCoreDNS(ctx, containerID); err != nil {
		log.Warnf("Failed to reload CoreDNS: %v", err)
	}

	log.Infof("Updated Tailscale DNS with %d node records", len(dnsRecords))
	return nil
}

// DNSRecord represents a DNS record for a containerlab node.
type DNSRecord struct {
	ShortName string
	LongName  string
	IPv4      string
	IPv6      string
}

// generateDNSRecords creates DNS records from the node map.
func (d *DockerRuntime) generateDNSRecords(labName string, nodes map[string]interface{}) []DNSRecord {
	var records []DNSRecord

	for _, nodeIface := range nodes {
		// Type assert to get node config
		// We expect nodes to have MgmtIPv4Address, MgmtIPv6Address, ShortName, LongName
		nodeConfig, ok := nodeIface.(interface {
			Config() *clabtypes.NodeConfig
		})
		if !ok {
			continue
		}

		cfg := nodeConfig.Config()
		if cfg == nil {
			continue
		}

		// Skip if node has no management IPs
		if cfg.MgmtIPv4Address == "" && cfg.MgmtIPv6Address == "" {
			continue
		}

		record := DNSRecord{
			ShortName: cfg.ShortName,
			LongName:  cfg.LongName,
			IPv4:      cfg.MgmtIPv4Address,
			IPv6:      cfg.MgmtIPv6Address,
		}

		records = append(records, record)
	}

	return records
}

// generateCorefile creates a CoreDNS Corefile configuration.
func (d *DockerRuntime) generateCorefile(labName string, records []DNSRecord) string {
	domain := fmt.Sprintf("%s.clab", labName)
	if d.mgmt.Tailscale.DNS != nil && d.mgmt.Tailscale.DNS.Domain != "" {
		domain = d.mgmt.Tailscale.DNS.Domain
	}

	// Prepare template data
	data := struct {
		LabName string
		Domain  string
	}{
		LabName: labName,
		Domain:  domain,
	}

	// Execute template
	tmpl, err := template.New("corefile").Parse(corefileTemplate)
	if err != nil {
		log.Warnf("Failed to parse Corefile template: %v", err)
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Warnf("Failed to execute Corefile template: %v", err)
		return ""
	}

	return buf.String()
}

// generateHostsFile creates a simple hosts file for CoreDNS.
// TODO: PTR (reverse DNS) records are not currently supported.
// CoreDNS hosts plugin can auto-generate PTR records, but proper configuration
// is needed to make it authoritative for reverse zones. This requires further
// investigation and testing with the hosts plugin's reverse DNS capabilities.
func (d *DockerRuntime) generateHostsFile(labName string, records []DNSRecord) string {
	domain := fmt.Sprintf("%s.clab", labName)
	if d.mgmt.Tailscale.DNS != nil && d.mgmt.Tailscale.DNS.Domain != "" {
		domain = d.mgmt.Tailscale.DNS.Domain
	}

	var hostsContent strings.Builder
	hostsContent.WriteString("# Containerlab DNS records\n")
	hostsContent.WriteString("# Generated automatically - do not edit\n\n")

	for _, record := range records {
		// Create FQDN
		fqdn := fmt.Sprintf("%s.%s", record.ShortName, domain)
		
		// Add IPv4 record
		if record.IPv4 != "" {
			hostsContent.WriteString(fmt.Sprintf("%s %s\n", record.IPv4, fqdn))
		}

		// Add IPv6 record
		if record.IPv6 != "" {
			hostsContent.WriteString(fmt.Sprintf("%s %s\n", record.IPv6, fqdn))
		}
	}

	return hostsContent.String()
}

// writeFileToContainer writes content to a file inside a container.
func (d *DockerRuntime) writeFileToContainer(ctx context.Context, containerID, path, content string) error {
	// Create directory if needed
	dirCmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p $(dirname %s)", path)}
	execConfig := container.ExecOptions{
		Cmd:          dirCmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := d.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec for mkdir: %w", err)
	}

	if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to execute mkdir: %w", err)
	}

	// Write file content
	writeCmd := []string{"sh", "-c", fmt.Sprintf("cat > %s", path)}
	execConfig = container.ExecOptions{
		Cmd:          writeCmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err = d.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec for write: %w", err)
	}

	// Attach to exec to send stdin
	attachResp, err := d.Client.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Write content to stdin
	if _, err := attachResp.Conn.Write([]byte(content)); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}
	attachResp.CloseWrite()

	// Start the exec
	if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start exec: %w", err)
	}

	return nil
}

// reloadCoreDNS sends a reload signal to CoreDNS running in the container.
func (d *DockerRuntime) reloadCoreDNS(ctx context.Context, containerID string) error {
	// Check if CoreDNS is running first
	checkCmd := []string{"pgrep", "coredns"}
	execConfig := container.ExecOptions{
		Cmd:          checkCmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := d.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to check if CoreDNS is running: %w", err)
	}

	if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to check if CoreDNS is running: %w", err)
	}

	inspectResp, err := d.Client.ContainerExecInspect(ctx, execID.ID)
	if err != nil || inspectResp.ExitCode != 0 {
		log.Debug("CoreDNS is not running, skipping reload")
		return nil
	}

	// Send SIGUSR1 to coredns process to reload configuration
	reloadCmd := []string{"pkill", "-USR1", "coredns"}
	execConfig = container.ExecOptions{
		Cmd:          reloadCmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err = d.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec for reload: %w", err)
	}

	if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to execute reload: %w", err)
	}

	log.Debug("CoreDNS configuration reloaded")
	return nil
}

// waitForTailscaleReady waits for Tailscale to be ready by checking its status.
func (d *DockerRuntime) waitForTailscaleReady(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		execConfig := container.ExecOptions{
			Cmd:          []string{"tailscale", "status", "--json"},
			AttachStdout: true,
			AttachStderr: true,
		}

		execID, err := d.Client.ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		inspectResp, err := d.Client.ContainerExecInspect(ctx, execID.ID)
		if err == nil && inspectResp.ExitCode == 0 {
			log.Debug("Tailscale is ready")
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("tailscale did not become ready within %v", timeout)
}

// waitForCoreDNSReady waits for CoreDNS process to be running.
func (d *DockerRuntime) waitForCoreDNSReady(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		execConfig := container.ExecOptions{
			Cmd:          []string{"pgrep", "coredns"},
			AttachStdout: true,
			AttachStderr: true,
		}

		execID, err := d.Client.ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if err := d.Client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		inspectResp, err := d.Client.ContainerExecInspect(ctx, execID.ID)
		if err == nil && inspectResp.ExitCode == 0 {
			log.Debug("CoreDNS is ready")
			return nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("coredns did not start within %v", timeout)
}
