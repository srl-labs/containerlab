package border0_api

import (
	"strings"
	"time"
)

type LoginResponse struct {
	LoginRefreshResponse
	MFA bool `json:"require_mfa"`
}

type LoginRefreshResponse struct {
	Token string `json:"token"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ErrorMessage struct {
	ErrorMessage string `json:"error_message,omitempty"`
}

type Socket struct {
	Tunnels                        []Tunnel          `json:"tunnels,omitempty"`
	Username                       string            `json:"user_name,omitempty"`
	SocketID                       string            `json:"socket_id,omitempty"`
	SocketTcpPorts                 []int             `json:"socket_tcp_ports,omitempty"`
	Dnsname                        string            `json:"dnsname,omitempty"`
	Name                           string            `json:"name,omitempty"`
	Description                    string            `json:"description,omitempty"`
	SocketType                     string            `json:"socket_type,omitempty"`
	ProtectedSocket                bool              `json:"protected_socket"`
	ProtectedUsername              string            `json:"protected_username"`
	ProtectedPassword              string            `json:"protected_password"`
	AllowedEmailAddresses          []string          `json:"cloud_authentication_email_allowed_addressses,omitempty"`
	AllowedEmailDomains            []string          `json:"cloud_authentication_email_allowed_domains,omitempty"`
	SSHCa                          string            `json:"ssh_ca,omitempty"`
	UpstreamUsername               string            `json:"upstream_username,omitempty"`
	UpstreamPassword               string            `json:"upstream_password,omitempty"`
	UpstreamHttpHostname           string            `json:"upstream_http_hostname,omitempty"`
	UpstreamType                   string            `json:"upstream_type,omitempty"`
	CloudAuthEnabled               bool              `json:"cloud_authentication_enabled,omitempty"`
	ConnectorAuthenticationEnabled bool              `json:"connector_authentication_enabled,omitempty"`
	Tags                           map[string]string `json:"tags,omitempty"`
	CustomDomains                  []string          `json:"custom_domains,omitempty"`
	PrivateSocket                  bool              `json:"private_socket"`
	PolicyNames                    []string          `json:"policy_names,omitempty"`
}

func (s *Socket) SanitizeName() {
	socketName := strings.ReplaceAll(s.Name, ".", "-")
	socketName = strings.ReplaceAll(socketName, " ", "-")
	socketName = strings.ReplaceAll(socketName, ".", "-")
	s.Name = strings.ReplaceAll(socketName, "_", "-")
}

type Tunnel struct {
	TunnelID     string `json:"tunnel_id,omitempty"`
	LocalPort    int    `json:"local_port,omitempty"`
	TunnelServer string `json:"tunnel_server,omitempty"`
}

type StaticSocketsConfig struct {
	Connector   *configConnector           `yaml:"connector"`
	Credentials *configCredentials         `yaml:"credentials"`
	Sockets     []map[string]*configSocket `yaml:"sockets"`
}

type configConnector struct {
	Name string `yaml:"name"`
}

type configCredentials struct {
	Token string `yaml:"token"`
}

type configSocket struct {
	Port     int      `yaml:"port"`
	Type     string   `yaml:"type"`
	Host     string   `yaml:"host"`
	Policies []string `yaml:"policies,omitempty"`
}

type CreatePolicyRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	PolicyData  PolicyData `json:"policy_data" binding:"required"`
	Orgwide     bool       `json:"org_wide"`
}

type UpdatePolicyRequest struct {
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	PolicyData  *PolicyData `json:"policy_data" binding:"required"`
}

type Policy struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	PolicyData  PolicyData `json:"policy_data"`
	SocketIDs   []string   `json:"socket_ids"`
	OrgID       string     `json:"org_id"`
	OrgWide     bool       `json:"org_wide"`
	CreatedAt   time.Time  `json:"created_at"`
}

type PolicyData struct {
	Version   string    `json:"version"`
	Action    []string  `json:"action" mapstructure:"action"`
	Condition Condition `json:"condition" mapstructure:"condition"`
}

type Condition struct {
	Who   ConditionWho   `json:"who,omitempty" mapstructure:"who"`
	Where ConditionWhere `json:"where,omitempty" mapstructure:"where"`
	When  ConditionWhen  `json:"when,omitempty" mapstructure:"when"`
}

type ConditionWho struct {
	Email  []string `json:"email,omitempty" mapstructure:"email"`
	Domain []string `json:"domain,omitempty" mapstructure:"domain"`
}

type ConditionWhere struct {
	AllowedIP  []string `json:"allowed_ip,omitempty" mapstructure:"allowed_ip"`
	Country    []string `json:"country,omitempty" mapstructure:"country"`
	CountryNot []string `json:"country_not,omitempty" mapstructure:"country_not"`
}

type ConditionWhat struct{}

type ConditionWhen struct {
	After           string `json:"after,omitempty" mapstructure:"after"`
	Before          string `json:"before,omitempty" mapstructure:"before"`
	TimeOfDayAfter  string `json:"time_of_day_after,omitempty" mapstructure:"time_of_day_after"`
	TimeOfDayBefore string `json:"time_of_day_before,omitempty" mapstructure:"time_of_day_before"`
}

type PolicyActionUpdateRequest struct {
	Action string `json:"action" binding:"required"`
	ID     string `json:"id" binding:"required"`
}
type AddSocketToPolicyRequest struct {
	Actions []PolicyActionUpdateRequest `json:"actions" binding:"required"`
}
