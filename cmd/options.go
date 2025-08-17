package cmd

import (
	"net"
	"time"

	clablinks "github.com/srl-labs/containerlab/links"
)

const (
	multiToolImage = "ghcr.io/srl-labs/network-multitool"
)

var optionsInstance *Options //nolint:gochecknoglobals

// GetOptions returns the global options instance if it exists
// or creates a new one with default values for all options.
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
				Host:           "localhost",
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
				CANamePrefix:     "ca",
				CertNamePrefix:   "cert",
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
