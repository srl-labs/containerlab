package cmd

import (
	"net"
	"os"
	"time"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	multiToolImage             = "ghcr.io/srl-labs/network-multitool"
	defaultTimeout             = 120 * time.Second
	defaultToolsServerPort     = 8080
	defaultToolsApiSSHBasePort = 2223
	defaultToolsApiSSHMaxPort  = 2322
	defaultToolsCertKeySize    = 2048
	defaultVxlanID             = 10
	defaultVxlanPort           = 14789
)

var optionsInstance *Options //nolint:gochecknoglobals

// GetOptions returns the global options instance if it exists
// or creates a new one with default values for all options.
func GetOptions() *Options {
	if optionsInstance == nil {
		optionsInstance = &Options{
			Global: &GlobalOptions{
				Timeout:  defaultTimeout,
				LogLevel: "info",
			},
			Filter: &FilterOptions{},
			Deploy: &DeployOptions{
				LabOwner: os.Getenv("CLAB_OWNER"),
			},
			Destroy: &DestroyOptions{},
			Config:  &ConfigOptions{},
			Exec: &ExecOptions{
				Format: "plain",
			},
			Inspect: &InspectOptions{
				Format:           "table",
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
				Port:           defaultToolsServerPort,
				Host:           "localhost",
				JWTExpiration:  "60m",
				UserGroup:      "clab_api",
				SuperUserGroup: "clab_admins",
				LogLevel:       "debug",
				GinMode:        "release",
				SSHBasePort:    defaultToolsApiSSHBasePort,
				SSHMaxPort:     defaultToolsApiSSHMaxPort,
				OutputFormat:   "table",
			},
			ToolsCert: &ToolsCertOptions{
				CommonName:       "containerlab.dev",
				Country:          "Internet",
				Locality:         "Server",
				Organization:     "Containerlab",
				OrganizationUnit: "Containerlab Tools",
				Expiry:           "87600h",
				KeySize:          defaultToolsCertKeySize,
				CANamePrefix:     "ca",
				CertNamePrefix:   "cert",
			},
			ToolsTxOffload: &ToolsDisableTxOffloadOptions{},
			ToolsGoTTY: &ToolsGoTTYOptions{
				Port:     defaultToolsServerPort,
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
				MTU: clabconstants.DefaultLinkMTU,
			},
			ToolsVxlan: &ToolsVxlanOptions{
				ID:             defaultVxlanID,
				Port:           defaultVxlanPort,
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

func (o *Options) ToClabOptions() []clabcore.ClabOption {
	var clabOptions []clabcore.ClabOption

	clabOptions = append(
		clabOptions,
		o.Global.toClabOptions()...,
	)

	clabOptions = append(
		clabOptions,
		o.Filter.toClabOptions()...,
	)

	clabOptions = append(
		clabOptions,
		o.Deploy.toClabOptions()...,
	)

	clabOptions = append(
		clabOptions,
		o.Destroy.toClabOptions()...,
	)

	return clabOptions
}

func (o *Options) ToClabDestroyOptions() []clabcore.DestroyOption {
	destroyOptions := []clabcore.DestroyOption{
		clabcore.WithDestroyMaxWorkers(o.Deploy.MaxWorkers),
		clabcore.WithDestroyNodeFilter(o.Filter.NodeFilter),
	}

	if o.Destroy.KeepManagementNetwork {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyKeepMgmtNet(),
		)
	}

	if o.Destroy.Cleanup {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyCleanup(),
		)
	}

	if o.Global.GracefulShutdown {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyGraceful(),
		)
	}

	if o.Destroy.All {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyAll(),
		)

		if !o.Destroy.AutoApprove {
			destroyOptions = append(
				destroyOptions,
				clabcore.WithDestroyTerminalPrompt(),
			)
		}
	}

	return destroyOptions
}

type GlobalOptions struct {
	TopologyFile     string
	VarsFile         string
	TopologyName     string
	Timeout          time.Duration
	Runtime          string
	GracefulShutdown bool
	LogLevel         string
	DebugCount       int

	// special flag that should only be set by deploy, informs the context handler to destroy
	// (or not) when root context is canceled
	CleanOnCancel bool
}

func (o *GlobalOptions) toClabOptions() []clabcore.ClabOption {
	options := []clabcore.ClabOption{
		clabcore.WithTimeout(o.Timeout),
		clabcore.WithRuntime(
			o.Runtime,
			&clabruntime.RuntimeConfig{
				Debug:            o.DebugCount > 0,
				Timeout:          o.Timeout,
				GracefulShutdown: o.GracefulShutdown,
			},
		),
		clabcore.WithDebug(o.DebugCount > 0),
	}

	if o.TopologyFile != "" {
		options = append(options, clabcore.WithTopoPath(o.TopologyFile, o.VarsFile))
	}

	if o.TopologyName != "" {
		options = append(options, clabcore.WithTopologyName(o.TopologyName))
	}

	return options
}

type FilterOptions struct {
	LabelFilter []string
	NodeFilter  []string
}

func (o *FilterOptions) toClabOptions() []clabcore.ClabOption {
	return []clabcore.ClabOption{
		clabcore.WithNodeFilter(o.NodeFilter),
	}
}

type DeployOptions struct {
	GenerateGraph            bool
	ManagementNetworkName    string
	ManagementIPv4Subnet     net.IPNet
	ManagementIPv6Subnet     net.IPNet
	Reconfigure              bool
	MaxWorkers               uint
	SkipPostDeploy           bool
	SkipLabDirectoryFileACLs bool
	ExportTemplate           string
	LabOwner                 string
}

func (o *DeployOptions) toClabOptions() []clabcore.ClabOption {
	options := []clabcore.ClabOption{
		clabcore.WithLabOwner(o.LabOwner),
	}

	if o.ManagementNetworkName != "" {
		options = append(options, clabcore.WithManagementNetworkName(o.ManagementNetworkName))
	}

	mgmtIPv4Subnet := o.ManagementIPv4Subnet.String()
	if mgmtIPv4Subnet != clabconstants.UnsetNetAddr {
		options = append(
			options,
			clabcore.WithManagementIpv4Subnet(mgmtIPv4Subnet),
		)
	}

	mgmtIPv6Subnet := o.ManagementIPv6Subnet.String()
	if mgmtIPv6Subnet != clabconstants.UnsetNetAddr {
		options = append(
			options,
			clabcore.WithManagementIpv6Subnet(mgmtIPv6Subnet),
		)
	}

	return options
}

type DestroyOptions struct {
	Cleanup               bool
	All                   bool
	KeepManagementNetwork bool
	AutoApprove           bool
}

func (o *DestroyOptions) toClabOptions() []clabcore.ClabOption {
	var options []clabcore.ClabOption

	if o.KeepManagementNetwork {
		options = append(options, clabcore.WithKeepMgmtNet())
	}

	return options
}

type ConfigOptions struct {
	TemplateVarOnly bool
}

type ExecOptions struct {
	Format   string
	Commands []string
}

type InspectOptions struct {
	Format           string
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
	CertNamePrefix   string
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
