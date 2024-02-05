package definitions

import "errors"

var ErrNotAvailabel = errors.New("not available")

const (
	DockerFWUserChain = "DOCKER-USER"
	DockerFWTable     = "filter"

	IPTablesRuleComment = "set by containerlab"

	IPTablesCommentMaxSize = 256
)

// ClabFirewall is the interface that all firewall clients must implement.
type ClabFirewall interface {
	DeleteForwardingRules() error
	InstallForwardingRules() error
	Name() string
}
