package definitions

import "errors"

var ErrNotAvailable = errors.New("not available")

const (
	DockerUserChain = "DOCKER-USER"
	ForwardChain    = "FORWARD"
	FilterTable     = "filter"

	IPTablesRuleComment = "set by containerlab"

	IPTablesCommentMaxSize = 256
)

// ClabFirewall is the interface that all firewall clients must implement.
type ClabFirewall interface {
	DeleteForwardingRules(inInterface, outInterface, chain string) error
	InstallForwardingRules(inInterface, outInterface, chain string) error
	Name() string
}
