package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kellerza/template"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

// templates to execute
var TemplateNames []string

// path to additional templates
var TemplatePath string

type NodeConfig struct {
	TargetNode *types.Node
	// All the variables used to render the template
	Vars map[string]interface{}
	// the Rendered templates
	Data []string
	Info []string
}

func RenderAll(nodes map[string]*types.Node, links map[int]*types.Link) (map[string]*NodeConfig, error) {
	res := make(map[string]*NodeConfig)

	tmpl, err := template.New("", template.SearchPath(TemplatePath))
	if err != nil {
		return nil, err
	}

	for nodeName, vars := range PrepareVars(nodes, links) {
		res[nodeName] = &NodeConfig{
			TargetNode: nodes[nodeName],
			Vars:       vars,
		}

		for _, baseN := range TemplateNames {
			tmplN := fmt.Sprintf("%s-%s.tmpl", baseN, vars["role"])
			data1, err := tmpl.ExecuteTemplate(tmplN, vars)
			if err != nil {
				return nil, err
			}
			data1 = strings.ReplaceAll(strings.Trim(data1, "\n \t"), "\n\n\n", "\n\n")
			res[nodeName].Data = append(res[nodeName].Data, data1)
			res[nodeName].Info = append(res[nodeName].Info, tmplN)
		}
	}
	return res, nil
}

// Implement stringer for conf snippet
func (c *NodeConfig) String() string {

	s := fmt.Sprintf("%s: %v", c.TargetNode.ShortName, c.Info)
	return s
}

// Print the config
func (c *NodeConfig) Print(printLines int) {
	var s strings.Builder

	s.WriteString(c.TargetNode.ShortName)

	if log.IsLevelEnabled(log.DebugLevel) {
		s.WriteString(" vars = ")
		vars, _ := json.MarshalIndent(c.Vars, "", "      ")
		s.Write(vars[0 : len(vars)-1])
		s.WriteString("  }")
	}

	if printLines > 0 {
		for idx, conf := range c.Data {
			fmt.Fprintf(&s, "\n  Template %s for %s = [[", c.Info[idx], c.TargetNode.ShortName)

			cl := strings.SplitN(conf, "\n", printLines+1)
			if len(cl) > printLines {
				cl[printLines] = "..."
			}
			for _, l := range cl {
				s.WriteString("\n     ")
				s.WriteString(l)
			}
			s.WriteString("\n  ]]")
		}
	}

	log.Infoln(s.String())
}
