package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"text/template"

	jT "github.com/kellerza/template"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

// templates to execute
var TemplateNames []string

// path to additional templates
var TemplatePaths []string

// debug count
var DebugCount int

type NodeConfig struct {
	TargetNode *types.NodeConfig
	// All the variables used to render the template
	Vars map[string]interface{}
	// the Rendered templates
	Data []string
	Info []string
}

// Load templates from all paths for the specific role/kind
func LoadTemplates(tmpl *template.Template, role string) error {
	for _, p := range TemplatePaths {
		fn := filepath.Join(p, fmt.Sprintf("*__%s.tmpl", role))
		_, err := tmpl.ParseGlob(fn)
		if err != nil {
			return fmt.Errorf("could not load templates from %s: %w", fn, err)
		}
	}
	return nil
}

func RenderAll(allnodes map[string]*NodeConfig) error {

	if len(TemplatePaths) == 0 { // default is the install path
		TemplatePaths = []string{"@"}
	}

	for i, v := range TemplatePaths {
		if v == "@" {
			TemplatePaths[i] = "/etc/containerlab/templates/"
		}
	}

	if len(TemplateNames) == 0 {
		var err error
		TemplateNames, err = GetTemplateNamesInDirs(TemplatePaths)
		if err != nil {
			return err
		}
		log.Infof("No template names specified (-l) using: %s", strings.Join(TemplateNames, ", "))
	}

	tmpl := template.New("").Funcs(jT.Funcs)

	for _, nc := range allnodes {
		for _, baseN := range TemplateNames {
			tmplN := fmt.Sprintf("%s__%s.tmpl", baseN, nc.Vars[vkRole])
			log.Debugf("Looking up template %v", tmplN)
			if l := tmpl.Lookup(tmplN); l == nil {
				err := LoadTemplates(tmpl, fmt.Sprintf("%s", nc.Vars[vkRole]))
				if err != nil {
					log.Warnf("Unable to load template %s (%+v); skipping", tmplN, err)
					continue
				}
				l = tmpl.Lookup(tmplN)
				log.Debugf("Got a lookup result %+v (of type %T)", l, l)
				if l == nil {
					log.Warnf("No template found for %s; skipping..", nc.TargetNode.ShortName)
					continue
				}
			}
			var buf strings.Builder
			err := tmpl.ExecuteTemplate(&buf, tmplN, nc.Vars)
			log.Debugf("Executed a template %s with an error code %v", tmplN, err)
			if err != nil {
				nc.Print(true, true)
				return err
			}

			data := strings.ReplaceAll(strings.Trim(buf.String(), "\n \t\r"), "\n\n\n", "\n\n")
			nc.Data = append(nc.Data, data)
			nc.Info = append(nc.Info, tmplN)
		}
	}
	return nil
}

// Implement stringer for NodeConfig
func (c *NodeConfig) String() string {
	s := fmt.Sprintf("%s: %v", c.TargetNode.ShortName, c.Info)
	return s
}

// Print the config
func (c *NodeConfig) Print(vars, rendered bool) { //skipcq: RVV-A0005
	var s strings.Builder

	s.WriteString(c.TargetNode.ShortName)

	if vars {
		s.WriteString(" vars = ")
		var saved_nodes Dict
		restore := false
		if DebugCount < 3 {
			saved_nodes, restore = c.Vars[vkNodes].(Dict)
			if restore {
				var n strings.Builder
				n.WriteRune('{')
				for k := range saved_nodes {
					fmt.Fprintf(&n, "%s: {...}, ", k)
				}
				n.WriteRune('}')
				c.Vars[vkNodes] = n.String()
			}
		}
		vars, err := yaml.Marshal(c.Vars)
		if err != nil {
			log.Warnf("error printing vars for node %s: %s", c.TargetNode.ShortName, err)
			s.WriteString(err.Error())
		}
		if restore {
			c.Vars[vkNodes] = saved_nodes
		}
		if len(vars) > 0 {
			s.Write(vars)
		}
	}

	if rendered {
		for idx, conf := range c.Data {
			fmt.Fprintf(&s, "\n  Template %s for %s = [[", c.Info[idx], c.TargetNode.ShortName)

			cl := strings.Split(conf, "\n")
			for _, l := range cl {
				s.WriteString("\n     ")
				s.WriteString(l)
			}
			s.WriteString("\n  ]]")
		}
	}

	log.Infoln(s.String())
}
