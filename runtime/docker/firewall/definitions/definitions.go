package definitions

import "errors"

var ErrNotAvailabel = errors.New("not available")

const (
	DockerFWUserChain = "DOCKER-USER"
	DockerFWTable     = "filter"

	IPTablesRuleComment = "set by containerlab"

	IPTablesCommentMaxSize = 256
)

type ClabFirewall interface {
	DeleteForwardingRules() error
	InstallForwardingRules() error
	Name() string
}
