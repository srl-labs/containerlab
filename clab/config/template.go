package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"text/template"

	jT "github.com/kellerza/template"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

// templates to execute
var TemplateNames []string

// path to additional templates
var TemplatePaths []string

type NodeConfig struct {
	TargetNode *types.NodeConfig
	// All the variables used to render the template
	Vars map[string]interface{}
	// the Rendered templates
	Data []string
	Info []string
}

func LoadTemplate(tmpl *template.Template, name string) error {
	for i := len(TemplatePaths) - 1; i >= 0; i-- {
		fn := filepath.Join(TemplatePaths[i], name)
		_, err := tmpl.ParseFiles(fn)
		if os.IsNotExist(err) { // try in next path
			continue
		}
		if err != nil {
			return fmt.Errorf("could not load template %s: %s", fn, err)
		}
		log.Debugf("template loaded %d. %s %s\n", i, name, fn)
		return nil
	}
	return fmt.Errorf("could not find template %s in search path", name)
}

func RenderAll(nodes map[string]nodes.Node, links map[int]*types.Link) (map[string]*NodeConfig, error) {
	res := make(map[string]*NodeConfig)

	if len(TemplatePaths) == 0 {
		return nil, fmt.Errorf("please specify one of more paths with --template-path")
	}

	if len(TemplateNames) == 0 {
		var err error
		TemplateNames, err = GetTemplateNamesInDirs(TemplatePaths)
		if err != nil {
			return nil, err
		}
		if len(TemplateNames) == 0 {
			return nil, fmt.Errorf("no templates files were found by %s path", TemplatePaths)
		}
	}

	tmpl := template.New("").Funcs(jT.Funcs)

	for nodeName, vars := range PrepareVars(nodes, links) {
		res[nodeName] = &NodeConfig{
			TargetNode: nodes[nodeName].Config(),
			Vars:       vars,
		}

		for _, baseN := range TemplateNames {
			tmplN := fmt.Sprintf("%s__%s.tmpl", baseN, vars["role"])

			if tmpl.Lookup(tmplN) == nil {
				err := LoadTemplate(tmpl, tmplN)
				if err != nil {
					return nil, err
				}
			}

			var buf strings.Builder
			err := tmpl.ExecuteTemplate(&buf, tmplN, vars)
			if err != nil {
				return nil, err
			}

			data1 := strings.ReplaceAll(strings.Trim(buf.String(), "\n \t"), "\n\n\n", "\n\n")
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
