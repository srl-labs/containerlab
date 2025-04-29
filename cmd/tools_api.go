// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	clabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

// Configuration variables for the API Server commands
var (
	apiServerImage          string
	apiServerName           string
	apiServerLabsDir        string
	apiServerPort           int
	apiServerHost           string
	apiServerJWTSecret      string
	apiServerJWTExpiration  string
	apiServerUserGroup      string
	apiServerSuperUserGroup string
	apiServerRuntime        string
	apiServerLogLevel       string
	apiServerGinMode        string
	apiServerTrustedProxies string
	apiServerTLSEnable      bool
	apiServerTLSCertFile    string
	apiServerTLSKeyFile     string
	apiServerSSHBasePort    int
	apiServerSSHMaxPort     int
	apiServerOwner          string
	outputFormatAPI         string
)

// APIServerListItem defines the structure for API server container info in JSON output
type APIServerListItem struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	LabsDir string `json:"labs_dir"`
	Runtime string `json:"runtime"`
	Owner   string `json:"owner"`
}

// APIServerNode implements runtime.Node interface for API server containers
type APIServerNode struct {
	config *types.NodeConfig
}

// generateRandomJWTSecret creates a random string for use as JWT secret
func generateRandomJWTSecret() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Encode as base64 string
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func init() {
	toolsCmd.AddCommand(apiServerCmd)
	apiServerCmd.AddCommand(apiServerStartCmd)
	apiServerCmd.AddCommand(apiServerStopCmd)
	apiServerCmd.AddCommand(apiServerStatusCmd)

	apiServerCmd.PersistentFlags().StringVarP(&outputFormatAPI, "format", "f", "table",
		"output format for 'status' command (table, json)")

	// Start command flags
	apiServerStartCmd.Flags().StringVarP(&apiServerImage, "image", "i", "ghcr.io/srl-labs/clab-api-server/clab-api-server:latest",
		"container image to use for API server")
	apiServerStartCmd.Flags().StringVarP(&apiServerName, "name", "n", "clab-api-server",
		"name of the API server container")
	apiServerStartCmd.Flags().StringVarP(&apiServerLabsDir, "labs-dir", "l", "",
		"directory to mount as shared labs directory")
	apiServerStartCmd.Flags().IntVarP(&apiServerPort, "port", "p", 8080,
		"port to expose the API server on")
	apiServerStartCmd.Flags().StringVarP(&apiServerHost, "host", "", "localhost",
		"host address for the API server")
	apiServerStartCmd.Flags().StringVarP(&apiServerJWTSecret, "jwt-secret", "", "",
		"JWT secret key for authentication (generated randomly if not provided)")
	apiServerStartCmd.Flags().StringVarP(&apiServerJWTExpiration, "jwt-expiration", "", "60m",
		"JWT token expiration time")
	apiServerStartCmd.Flags().StringVarP(&apiServerUserGroup, "user-group", "", "clab_api",
		"user group for API access")
	apiServerStartCmd.Flags().StringVarP(&apiServerSuperUserGroup, "superuser-group", "", "clab_admins",
		"superuser group name")
	apiServerStartCmd.Flags().StringVarP(&apiServerLogLevel, "log-level", "", "debug",
		"log level (debug/info/warn/error)")
	apiServerStartCmd.Flags().StringVarP(&apiServerGinMode, "gin-mode", "", "release",
		"Gin framework mode (debug/release/test)")
	apiServerStartCmd.Flags().StringVarP(&apiServerTrustedProxies, "trusted-proxies", "", "",
		"comma-separated list of trusted proxies")
	apiServerStartCmd.Flags().BoolVarP(&apiServerTLSEnable, "tls-enable", "", false,
		"enable TLS for the API server")
	apiServerStartCmd.Flags().StringVarP(&apiServerTLSCertFile, "tls-cert", "", "",
		"path to TLS certificate file")
	apiServerStartCmd.Flags().StringVarP(&apiServerTLSKeyFile, "tls-key", "", "",
		"path to TLS key file")
	apiServerStartCmd.Flags().IntVarP(&apiServerSSHBasePort, "ssh-base-port", "", 2223,
		"SSH proxy base port")
	apiServerStartCmd.Flags().IntVarP(&apiServerSSHMaxPort, "ssh-max-port", "", 2322,
		"SSH proxy maximum port")
	apiServerStartCmd.Flags().StringVarP(&apiServerOwner, "owner", "o", "",
		"owner name for the API server container")

	// Removed: apiServerStartCmd.MarkFlagRequired("jwt-secret")

	// Stop command flags
	apiServerStopCmd.Flags().StringVarP(&apiServerName, "name", "n", "clab-api-server",
		"name of the API server container to stop")
}

// apiServerCmd represents the api-server command container
var apiServerCmd = &cobra.Command{
	Use:   "api-server",
	Short: "Containerlab API server operations",
	Long:  "Start, stop, and manage Containerlab API server containers",
}

func NewAPIServerNode(name, image, labsDir, runtime string, env map[string]string, labels map[string]string) *APIServerNode {
	log.Debugf("Creating APIServerNode: name=%s, image=%s, labsDir=%s, runtime=%s", name, image, labsDir, runtime)

	// Set up binds based on the runtime
	var binds []string

	switch runtime {
	case "podman":
		binds = []string{
			// Mount podman socket to its proper location
			"/run/podman/podman.sock:/run/podman/podman.sock",
			"/var/run/netns:/var/run/netns",
			"/var/lib/containers:/var/lib/containers",
			"/etc/passwd:/etc/passwd:ro",
			"/etc/shadow:/etc/shadow:ro",
			"/etc/group:/etc/group:ro",
			"/etc/gshadow:/etc/gshadow:ro",
			"/home:/home",
		}

		// For rootless podman, check if XDG_RUNTIME_DIR is set
		if os.Getenv("XDG_RUNTIME_DIR") != "" {
			userID := os.Getenv("UID")
			if userID == "" {
				userID = strconv.Itoa(os.Getuid())
			}
			podmanSocket := fmt.Sprintf("/run/user/%s/podman/podman.sock", userID)
			if _, err := os.Stat(podmanSocket); err == nil {
				// Mount the user-specific podman socket
				binds[0] = fmt.Sprintf("%s:/run/user/%s/podman/podman.sock", podmanSocket, userID)
			}
		}

	default: // docker
		binds = []string{
			"/var/run/docker.sock:/var/run/docker.sock",
			"/var/run/netns:/var/run/netns",
			"/var/lib/docker/containers:/var/lib/docker/containers",
			"/etc/passwd:/etc/passwd:ro",
			"/etc/shadow:/etc/shadow:ro",
			"/etc/group:/etc/group:ro",
			"/etc/gshadow:/etc/gshadow:ro",
			"/home:/home",
		}
	}

	// Find containerlab binary and add bind mount if found
	clabPath, err := findContainerlabPath()
	if err == nil {
		binds = append(binds, fmt.Sprintf("%s:/usr/bin/containerlab:ro", clabPath))
	} else {
		log.Warnf("Could not find containerlab binary: %v. API server might not function correctly if containerlab is not in its PATH.", err)
	}

	nodeConfig := &types.NodeConfig{
		LongName:    name,
		ShortName:   name,
		Image:       image,
		Env:         env,
		Binds:       binds,
		Labels:      labels,
		NetworkMode: "host", // Use host network namespace
	}

	return &APIServerNode{
		config: nodeConfig,
	}
}

// findContainerlabPath tries to find the containerlab binary path
func findContainerlabPath() (string, error) {
	// Try using 'which' command to locate containerlab
	cmd := exec.Command("which", "containerlab")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// Try common locations
	locations := []string{
		"/usr/bin/containerlab",
		"/usr/local/bin/containerlab",
		"/opt/containerlab/containerlab",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc, nil
		}
	}

	return "", fmt.Errorf("containerlab binary not found")
}

func (n *APIServerNode) Config() *types.NodeConfig {
	return n.config
}

// GetEndpoints implementation for the Node interface
func (n *APIServerNode) GetEndpoints() []links.Endpoint {
	return nil
}

// createLabels creates container labels
func createAPIServerLabels(containerName, owner string, port int, labsDir, host, runtimeType string) map[string]string {
	labels := map[string]string{
		"clab-node-name": containerName,
		"clab-node-kind": "linux",
		"clab-node-type": "tool",
		"tool-type":      "api-server",
		"clab-api-port":  fmt.Sprintf("%d", port),
		"clab-api-host":  host,
		"clab-labs-dir":  labsDir,
		"clab-runtime":   runtimeType,
	}

	// Add owner label if available
	if owner != "" {
		labels[clabels.Owner] = owner
	}

	return labels
}

// getOwnerName gets owner name from flag or environment variables
func getOwnerName() string {
	if apiServerOwner != "" {
		return apiServerOwner
	}

	if owner := os.Getenv("SUDO_USER"); owner != "" {
		return owner
	}

	return os.Getenv("USER")
}

// apiServerStartCmd starts API server container
var apiServerStartCmd = &cobra.Command{
	Use:     "start",
	Short:   "start Containerlab API server container",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("api-server start called with flags: name='%s', image='%s', labsDir='%s', port=%d, host='%s'",
			apiServerName, apiServerImage, apiServerLabsDir, apiServerPort, apiServerHost)

		// Generate random JWT secret if not provided
		if apiServerJWTSecret == "" {
			var err error
			apiServerJWTSecret, err = generateRandomJWTSecret()
			if err != nil {
				return fmt.Errorf("failed to generate random JWT secret: %w", err)
			}
			log.Infof("Generated random JWT secret for API server")
		}

		runtimeName := common.Runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(runtimeName)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer for '%s': %w", runtimeName, err)
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Check if container already exists
		filter := []*types.GenericFilter{{FilterType: "name", Match: apiServerName}}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) > 0 {
			return fmt.Errorf("container %s already exists", apiServerName)
		}

		// Pull the container image
		log.Infof("Pulling image %s...", apiServerImage)
		if err := rt.PullImage(ctx, apiServerImage, types.PullPolicyAlways); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", apiServerImage, err)
		}

		// Create environment variables map
		env := map[string]string{
			"CLAB_SHARED_LABS_DIR":   apiServerLabsDir,
			"API_PORT":               fmt.Sprintf("%d", apiServerPort),
			"API_SERVER_HOST":        apiServerHost,
			"JWT_SECRET":             apiServerJWTSecret,
			"JWT_EXPIRATION_MINUTES": apiServerJWTExpiration,
			"API_USER_GROUP":         apiServerUserGroup,
			"SUPERUSER_GROUP":        apiServerSuperUserGroup,
			"CLAB_RUNTIME":           apiServerRuntime,
			"LOG_LEVEL":              apiServerLogLevel,
			"GIN_MODE":               apiServerGinMode,
		}

		// Add optional environment variables
		if apiServerTrustedProxies != "" {
			env["TRUSTED_PROXIES"] = apiServerTrustedProxies
		}
		if apiServerTLSEnable {
			env["TLS_ENABLE"] = "true"
			if apiServerTLSCertFile != "" {
				env["TLS_CERT_FILE"] = apiServerTLSCertFile
			}
			if apiServerTLSKeyFile != "" {
				env["TLS_KEY_FILE"] = apiServerTLSKeyFile
			}
		}
		if apiServerSSHBasePort > 0 {
			env["SSH_BASE_PORT"] = fmt.Sprintf("%d", apiServerSSHBasePort)
		}
		if apiServerSSHMaxPort > 0 {
			env["SSH_MAX_PORT"] = fmt.Sprintf("%d", apiServerSSHMaxPort)
		}

		// Create container labels
		if apiServerLabsDir == "" {
			apiServerLabsDir = "~/.clab"
		}
		owner := getOwnerName()
		labels := createAPIServerLabels(apiServerName, owner, apiServerPort, apiServerLabsDir, apiServerHost, runtimeName)

		// Create and start API server container
		log.Infof("Creating API server container %s", apiServerName)
		apiServerNode := NewAPIServerNode(apiServerName, apiServerImage, apiServerLabsDir, runtimeName, env, labels)

		id, err := rt.CreateContainer(ctx, apiServerNode.Config())
		if err != nil {
			return fmt.Errorf("failed to create API server container: %w", err)
		}

		if _, err := rt.StartContainer(ctx, id, apiServerNode); err != nil {
			// Clean up on failure
			rt.DeleteContainer(ctx, apiServerName)
			return fmt.Errorf("failed to start API server container: %w", err)
		}

		log.Infof("API server container %s started successfully.", apiServerName)
		log.Infof("API Server available at: http://%s:%d", apiServerHost, apiServerPort)
		if apiServerTLSEnable {
			log.Infof("API Server TLS enabled at: https://%s:%d", apiServerHost, apiServerPort)
		}

		return nil
	},
}

var apiServerStopCmd = &cobra.Command{
	Use:     "stop",
	Short:   "stop Containerlab API server container",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Debugf("Container name for deletion: %s", apiServerName)

		// Use common.Runtime if available, otherwise use the api-server flag
		runtimeName := common.Runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize runtime
		_, rinit, err := clab.RuntimeInitializer(runtimeName)
		if err != nil {
			return fmt.Errorf("failed to get runtime initializer: %w", err)
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: common.Timeout}))
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		log.Infof("Removing API server container %s", apiServerName)
		if err := rt.DeleteContainer(ctx, apiServerName); err != nil {
			return fmt.Errorf("failed to remove API server container: %w", err)
		}

		log.Infof("API server container %s removed successfully", apiServerName)
		return nil
	},
}

// apiServerStatusCmd shows status of active API server containers
var apiServerStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "show status of active Containerlab API server containers",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Use common.Runtime for consistency with other commands
		runtimeName := common.Runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize containerlab with runtime using the same approach as inspect command
		opts := []clab.ClabOption{
			clab.WithTimeout(common.Timeout),
			clab.WithRuntime(runtimeName,
				&runtime.RuntimeConfig{
					Debug:            common.Debug,
					Timeout:          common.Timeout,
					GracefulShutdown: common.Graceful,
				},
			),
			clab.WithDebug(common.Debug),
		}

		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		// Check connectivity like inspect does
		err = c.CheckConnectivity(ctx)
		if err != nil {
			return err
		}

		// Filter only by API server label
		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      "tool-type",
				Operator:   "=",
				Match:      "api-server",
			},
		}

		containers, err := c.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) == 0 {
			if outputFormatAPI == "json" {
				fmt.Println("[]")
			} else {
				fmt.Println("No active API server containers found")
			}
			return nil
		}

		// Process containers and format output
		listItems := make([]APIServerListItem, 0, len(containers))
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")

			// Get port from labels or use default
			port := 8080 // default
			if portStr, ok := c.Labels["clab-api-port"]; ok {
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				}
			}

			// Get host from labels or use default
			host := "localhost" // default
			if hostVal, ok := c.Labels["clab-api-host"]; ok {
				host = hostVal
			}

			// Get labs dir from labels or use default
			labsDir := "~/.clab" // default
			if dirsVal, ok := c.Labels["clab-labs-dir"]; ok {
				labsDir = dirsVal
			}

			// Get runtime from labels or use default
			runtimeType := "docker" // default
			if rtVal, ok := c.Labels["clab-runtime"]; ok {
				runtimeType = rtVal
			}

			// Get owner from container labels
			owner := "N/A"
			if ownerVal, exists := c.Labels[clabels.Owner]; exists && ownerVal != "" {
				owner = ownerVal
			}

			listItems = append(listItems, APIServerListItem{
				Name:    name,
				State:   c.State,
				Host:    host,
				Port:    port,
				LabsDir: labsDir,
				Runtime: runtimeType,
				Owner:   owner,
			})
		}

		// Output based on format
		if outputFormatAPI == "json" {
			b, err := json.MarshalIndent(listItems, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal to JSON: %w", err)
			}
			fmt.Println(string(b))
		} else {
			// Use go-pretty table
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleRounded)
			t.Style().Format.Header = text.FormatTitle
			t.Style().Options.SeparateRows = true

			t.AppendHeader(table.Row{"NAME", "STATUS", "HOST", "PORT", "LABS DIR", "RUNTIME", "OWNER"})

			for _, item := range listItems {
				t.AppendRow(table.Row{
					item.Name,
					item.State,
					item.Host,
					item.Port,
					item.LabsDir,
					item.Runtime,
					item.Owner,
				})
			}
			t.Render()
		}

		return nil
	},
}
