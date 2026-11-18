package definitions

import (
	"context"
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
	DeleteForwardingRules(ctx context.Context, rule *FirewallRule) error
	InstallForwardingRules(ctx context.Context, rule *FirewallRule) error
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
