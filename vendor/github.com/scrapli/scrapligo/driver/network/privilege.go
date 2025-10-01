package network

import (
	"regexp"
	"strings"
)

// PrivilegeLevels is a type alias for the map of privilege levels that gets assigned to a network
// Driver object.
type PrivilegeLevels map[string]*PrivilegeLevel

// PrivilegeLevel defines a privilege level, including a name, the pattern used to match a prompt
// output to the privilege level, as well as information about how to escalate into and deescalate
// out of this privilege level.
type PrivilegeLevel struct {
	Name           string `yaml:"name"`
	Pattern        string `yaml:"pattern"`
	patternRe      *regexp.Regexp
	NotContains    []string `yaml:"not-contains"`
	PreviousPriv   string   `yaml:"previous-priv"`
	Deescalate     string   `yaml:"deescalate"`
	Escalate       string   `yaml:"escalate"`
	EscalateAuth   bool     `yaml:"escalate-auth"`
	EscalatePrompt string   `yaml:"escalate-prompt"`
}

func (d *Driver) buildPrivGraph() {
	d.privGraph = map[string]map[string]bool{}

	for _, privLevel := range d.PrivilegeLevels {
		privLevel.patternRe = regexp.MustCompile(privLevel.Pattern)
		d.privGraph[privLevel.Name] = map[string]bool{}

		if privLevel.PreviousPriv != "" {
			d.privGraph[privLevel.Name][privLevel.PreviousPriv] = true
		}
	}

	for higherPrivLevel, privLevelList := range d.privGraph {
		for privLevel := range privLevelList {
			d.privGraph[privLevel][higherPrivLevel] = true
		}
	}
}

func (d *Driver) buildJoinedPromptPattern() {
	patterns := make([]string, 0)

	for _, priv := range d.PrivilegeLevels {
		patterns = append(patterns, priv.Pattern)
	}

	joinedPattern := strings.Join(patterns, "|")

	d.Driver.Channel.PromptPattern = regexp.MustCompile(joinedPattern)
}

// UpdatePrivileges refreshes the Driver's internal privilege map, the map that is used to determine
// appropriate next steps during privilege escalation/deescalation. Any time a user modifies the
// Driver PrivilegeLevels this method should be called as it will regenerate the base
// channel.Channel prompt pattern to include all privilege level patterns in the PrivilegeLevels
// map, thus ensuring we can always "find" a prompt.
func (d *Driver) UpdatePrivileges() {
	d.buildPrivGraph()
	d.buildJoinedPromptPattern()
}
