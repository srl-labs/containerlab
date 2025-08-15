// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabgit "github.com/srl-labs/containerlab/git"
	clablinks "github.com/srl-labs/containerlab/links"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	multiToolImage = "ghcr.io/srl-labs/network-multitool"
)

var optionsInstance *Options //nolint:gochecknoglobals

func GetOptions() *Options {
	if optionsInstance == nil {
		optionsInstance = &Options{
			Global: &GlobalOptions{
				Timeout:  120 * time.Second,
				LogLevel: "info",
			},
			Filter: &FilterOptions{},
			Deploy: &DeployOptions{
				Format: "table",
			},
			Destroy: &DestroyOptions{},
			Config:  &ConfigOptions{},
			Exec: &ExecOptions{
				Format: "plain",
			},
			Inspect: &InspectOptions{
				InterfacesFormat: "table",
			},
			Graph: &GraphOptions{
				Server:           "0.0.0.0:50080",
				MermaidDirection: "TD",
				DrawIOVersion:    "latest",
			},
			ToolsAPI: &ToolsApiOptions{
				Image:          "ghcr.io/srl-labs/clab-api-server/clab-api-server:latest",
				Name:           "clab-api-server",
				Port:           8080,
				Host:           "localshost",
				JWTExpiration:  "60m",
				UserGroup:      "clab_api",
				SuperUserGroup: "clab_admins",
				Runtime:        "docker",
				LogLevel:       "debug",
				GinMode:        "release",
				SSHBasePort:    2223,
				SSHMaxPort:     2322,
				OutputFormat:   "table",
			},
			ToolsCert: &ToolsCertOptions{
				CommonName:       "containerlab.dev",
				Country:          "Internet",
				Locality:         "Server",
				Organization:     "Containerlab",
				OrganizationUnit: "Containerlab Tools",
				Expiry:           "87600h",
				KeySize:          2048,
			},
			ToolsTxOffload: &ToolsDisableTxOffloadOptions{},
			ToolsGoTTY: &ToolsGoTTYOptions{
				Port:     8080,
				Username: "admin",
				Password: "admin",
				Shell:    "bash",
				Image:    multiToolImage,
				Format:   "table",
			},
			ToolsNetem: &ToolsNetemOptions{
				Format: "table",
			},
			ToolsSSHX: &ToolsSSHXOptions{
				Image:  multiToolImage,
				Format: "table",
			},
			ToolsVeth: &ToolsVethOptions{
				MTU: clablinks.DefaultLinkMTU,
			},
			ToolsVxlan: &ToolsVxlanOptions{
				ID:             10,
				Port:           14789,
				DeletionPrefix: "vx-",
			},
		}
	}

	return optionsInstance
}

type Options struct {
	Global         *GlobalOptions
	Filter         *FilterOptions
	Deploy         *DeployOptions
	Destroy        *DestroyOptions
	Config         *ConfigOptions
	Exec           *ExecOptions
	Inspect        *InspectOptions
	Graph          *GraphOptions
	ToolsAPI       *ToolsApiOptions
	ToolsCert      *ToolsCertOptions
	ToolsTxOffload *ToolsDisableTxOffloadOptions
	ToolsGoTTY     *ToolsGoTTYOptions
	ToolsNetem     *ToolsNetemOptions
	ToolsSSHX      *ToolsSSHXOptions
	ToolsVeth      *ToolsVethOptions
	ToolsVxlan     *ToolsVxlanOptions
}

type GlobalOptions struct {
	TopologyFile string
	VarsFile     string
	TopologyName string
	Timeout      time.Duration
	Runtime      string
	LogLevel     string
	DebugCount   int
}

type FilterOptions struct {
	LabelFilter []string
	NodeFilter  []string
}

type DeployOptions struct {
	GenerateGraph            bool
	ManagementNetworkName    string
	ManagementIPv4Subnet     net.IPNet
	ManagementIPv6Subnet     net.IPNet
	Format                   string
	Reconfigure              bool
	MaxWorkers               uint
	SkipPostDeploy           bool
	SkipLabDirectoryFileACLs bool
	ExportTemplate           string
	LabOwner                 string
}

type DestroyOptions struct {
	Cleanup               bool
	All                   bool
	GracefulShutdown      bool
	KeepManagementNetwork bool
	AutoApprove           bool
}

type ConfigOptions struct {
	TemplateVarOnly bool
}

type ExecOptions struct {
	Format   string
	Commands []string
}

type InspectOptions struct {
	Details          bool
	Wide             bool
	InterfacesFormat string
	InterfacesNode   string
}

type GraphOptions struct {
	Server           string
	Template         string
	Offline          bool
	GenerateDotFile  bool
	GenerateMermaid  bool
	MermaidDirection string
	GenerateDrawIO   bool
	DrawIOVersion    string
	DrawIOArgs       []string
	StaticDirectory  string
}

type ToolsApiOptions struct {
	Image          string
	Name           string
	Port           uint
	Host           string
	LabsDirectory  string
	JWTSecret      string
	JWTExpiration  string
	UserGroup      string
	SuperUserGroup string
	Runtime        string
	LogLevel       string
	GinMode        string
	TrustedProxies string
	TLSEnable      bool
	TLSCertFile    string
	TLSKeyFile     string
	SSHBasePort    uint
	SSHMaxPort     uint
	Owner          string
	OutputFormat   string
}

type ToolsCertOptions struct {
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           string
	Path             string
	CANamePrefix     string
	CertHosts        []string
	CACertPath       string
	CAKeyPath        string
	KeySize          uint
}

type ToolsDisableTxOffloadOptions struct {
	ContainerName string
}

type ToolsGoTTYOptions struct {
	ContainerName string
	Port          uint
	Username      string
	Password      string
	Shell         string
	Image         string
	Format        string
	Owner         string
}

type ToolsNetemOptions struct {
	ContainerName string
	Interface     string
	Delay         time.Duration
	Jitter        time.Duration
	Loss          float64
	Rate          uint64
	Corruption    float64
	Format        string
}

type ToolsSSHXOptions struct {
	ContainerName string
	EnableReaders bool
	Image         string
	Owner         string
	MountSSHDir   bool
	Format        string
}

type ToolsVethOptions struct {
	AEndpoint string
	BEndpoint string
	MTU       int
}

type ToolsVxlanOptions struct {
	Link           string
	ID             uint
	MTU            uint
	Port           uint
	Remote         string
	ParentDevice   string
	DeletionPrefix string
}

func subcommandRegisterFuncs() []func(*Options) (*cobra.Command, error) {
	return []func(*Options) (*cobra.Command, error){
		versionCmd,
		completionCmd,
		configCmd,
		deployCmd,
		destroyCmd,
		execCmd,
		generateCmd,
		graphCmd,
		inspectCmd,
		redeployCmd,
		saveCmd,
		toolsCmd,
	}
}

func Entrypoint() (*cobra.Command, error) {
	o := GetOptions()

	c := &cobra.Command{
		Use:   "containerlab",
		Short: "deploy container based lab environments with a user-defined interconnections",
		PersistentPreRunE: func(cobraCmd *cobra.Command, args []string) error {
			return preRunFn(cobraCmd, o)
		},
		Aliases:      []string{"clab"},
		SilenceUsage: true,
	}

	c.PersistentFlags().CountVarP(&o.Global.DebugCount, "debug", "d", "enable debug mode")
	c.PersistentFlags().StringVarP(&o.Global.TopologyFile, "topo", "t", "",
		"path to the topology definition file, a directory containing one, 'stdin', or a URL")
	c.PersistentFlags().StringVarP(&o.Global.VarsFile, "vars", "", "",
		"path to the topology template variables file")
	c.PersistentFlags().StringVarP(&o.Global.TopologyName, "name", "", "", "lab/topology name")
	c.PersistentFlags().DurationVarP(&o.Global.Timeout, "timeout", "", o.Global.Timeout,
		"timeout for external API requests (e.g. container runtimes), e.g: 30s, 1m, 2m30s")
	c.PersistentFlags().StringVarP(&o.Global.Runtime, "runtime", "r", "", "container runtime")
	c.PersistentFlags().StringVarP(&o.Global.LogLevel, "log-level", "", o.Global.LogLevel,
		"logging level; one of [trace, debug, info, warning, error, fatal]")

	err := c.MarkPersistentFlagFilename("topo", "*.yaml", "*.yml")
	if err != nil {
		return nil, err
	}

	for _, f := range subcommandRegisterFuncs() {
		cmd, err := f(o)
		if err != nil {
			return nil, err
		}

		c.AddCommand(cmd)
	}

	return c, nil
}

func preRunFn(cobraCmd *cobra.Command, o *Options) error {
	// setting log level
	switch {
	case o.Global.DebugCount > 0:
		log.SetLevel(log.DebugLevel)
	default:
		l, err := log.ParseLevel(o.Global.LogLevel)
		if err != nil {
			return err
		}

		log.SetLevel(l)
	}

	// initializes the version manager that goes off and fetches current version in
	// the background for us
	initVersionManager(cobraCmd.Context())

	// setting output to stderr, so that json outputs can be parsed
	log.SetOutput(os.Stderr)

	log.SetTimeFormat(time.TimeOnly)

	err := clabutils.DropRootPrivs()
	if err != nil {
		return err
	}
	// Rootless operations only supported for Docker runtime
	if o.Global.Runtime != "" && o.Global.Runtime != "docker" {
		err := clabutils.CheckAndGetRootPrivs(cobraCmd, nil)
		if err != nil {
			return err
		}
	}

	return getTopoFilePath(cobraCmd, o)
}

// getTopoFilePath finds *.clab.y*ml file in the current working directory
// if the file was not specified.
// If the topology file refers to a git repository, it will be cloned to the current directory.
// Errors if more than one file is found by the glob path.
func getTopoFilePath(cobraCmd *cobra.Command, o *Options) error { // skipcq: GO-R1005
	// set commands which may use topo file find functionality, the rest don't need it
	if cobraCmd.Name() != "deploy" &&
		cobraCmd.Name() != "destroy" &&
		cobraCmd.Name() != "redeploy" &&
		cobraCmd.Name() != "inspect" &&
		cobraCmd.Name() != "save" &&
		cobraCmd.Name() != "graph" &&
		cobraCmd.Name() != "interfaces" {
		return nil
	}

	// inspect and destroy commands with --all flag don't use file find functionality
	if (cobraCmd.Name() == "inspect" || cobraCmd.Name() == "destroy") &&
		cobraCmd.Flag("all").Value.String() == "true" {
		return nil
	}

	var err error
	// perform topology clone/fetch if the topo file is not available locally
	if !clabutils.FileOrDirExists(o.Global.TopologyFile) {
		switch {
		case clabgit.IsGitHubOrGitLabURL(o.Global.TopologyFile) ||
			clabgit.IsGitHubShortURL(o.Global.TopologyFile):
			o.Global.TopologyFile, err = processGitTopoFile(o.Global.TopologyFile)
			if err != nil {
				return err
			}
		case clabutils.IsHttpURL(o.Global.TopologyFile, true):
			// canonize the passed topo as URL by adding https schema if it was missing
			if !strings.HasPrefix(o.Global.TopologyFile, "http://") &&
				!strings.HasPrefix(o.Global.TopologyFile, "https://") {
				o.Global.TopologyFile = "https://" + o.Global.TopologyFile
			}
		}
	}

	// if topo or name flags have been provided, don't try to derive the topo file
	if o.Global.TopologyFile != "" || o.Global.TopologyName != "" {
		return nil
	}

	log.Debugf("trying to find topology files automatically")

	files, err := filepath.Glob("*.clab.y*ml")

	if len(files) == 0 {
		return errors.New("no topology files matching the pattern *.clab.yml or *.clab.yaml found")
	}

	if len(files) > 1 {
		return fmt.Errorf("more than one topology file matching the pattern *.clab.yml or *.clab.yaml found, can't pick one: %q", files)
	}

	o.Global.TopologyFile = files[0]

	log.Debugf("topology file found: %s", files[0])

	return err
}

func processGitTopoFile(topo string) (string, error) {
	// for short github urls, prepend https://github.com
	// note that short notation only works for github links
	if clabgit.IsGitHubShortURL(topo) {
		topo = "https://github.com/" + topo
	}

	repo, err := clabgit.NewRepo(topo)
	if err != nil {
		return "", err
	}

	// Instantiate the git implementation to use.
	gitImpl := clabgit.NewGoGit(repo)

	// clone the repo via the Git Implementation
	err = gitImpl.Clone()
	if err != nil {
		return "", err
	}

	// adjust permissions for the checked out repo
	// it would belong to root/root otherwise
	err = clabutils.SetUIDAndGID(repo.GetName())
	if err != nil {
		log.Errorf("error adjusting repository permissions %v. Continuing anyways", err)
	}

	// prepare the path with the repo based path
	path := filepath.Join(repo.GetPath()...)
	// prepend that path with the repo base directory
	path = filepath.Join(repo.GetName(), path)

	// change dir to the
	err = os.Chdir(path)
	if err != nil {
		return "", err
	}

	return repo.GetFilename(), err
}
