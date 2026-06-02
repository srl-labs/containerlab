package definitions

import (
	"errors"
)

var ErrNotAvailable = errors.New("not available")

const (
	DockerUserChain = "DOCKER-USER"
	ForwardChain    = "FORWARD"
	FilterTable     = "filter"
	AcceptAction    = "ACCEPT"
	InDirection     = "in"
	OutDirection    = "out"

	ContainerlabComment = "set by containerlab"

	IPTablesCommentMaxSize = 256
)

// ClabFirewall is the interface that all firewall clients must implement.
type ClabFirewall interface {
	DeleteForwardingRules(rule *FirewallRule) error
	InstallForwardingRules(rule *FirewallRule) error
	Name() string
}

type FirewallRule struct {
	Chain     string
	Table     string
	Interface string
	Direction string
	Action    string
	Comment   string
}
