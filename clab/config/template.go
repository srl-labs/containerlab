package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab"
)

type ConfigSnippet struct {
	TargetNode *clab.Node
	// the Rendered template
	Data []byte
	// some info for tracing/debugging
	templateName, source string
	// All the variables used to render the template
	vars *map[string]string
}

// internal template cache
var templates map[string]*template.Template

func LoadTemplate(kind, templatePath string) error {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	if _, ok := templates[kind]; ok {
		return nil
	}

	tp := filepath.Join(templatePath, kind, "*.tmpl")
	log.Debugf("Load templates from: %s", tp)

	ct := template.New(kind).Funcs(funcMap)
	var err error
	templates[kind], err = ct.ParseGlob(tp)
	if err != nil {
		log.Errorf("could not load template %s", err)
		return err
	}
	return nil
}

func (c *ConfigSnippet) Render() error {
	t := templates[c.TargetNode.Kind]
	buf := new(strings.Builder)
	c.Data = nil

	varsP, err := json.MarshalIndent(c.vars, "", "  ")
	if err != nil {
		varsP = []byte(fmt.Sprintf("%s", c.vars))
	}

	err = t.ExecuteTemplate(buf, c.templateName, c.vars)
	if err != nil {
		log.Errorf("could not render template %s: %s vars=%s\n", c.String(), err, varsP)
		return fmt.Errorf("could not render template %s: %s", c.String(), err)
	}

	// Strip blank lines
	res := strings.Trim(buf.String(), "\n")
	res = strings.ReplaceAll(res, "\n\n\n", "\n\n")
	c.Data = []byte(res)

	return nil
}

func RenderNode(node *clab.Node) ([]ConfigSnippet, error) {
	snips := []ConfigSnippet{}
	nc := GetNodeConfigFromLabels(node.Labels)

	for _, tn := range nc.Templates {
		tn = fmt.Sprintf("%s-node.tmpl", tn)
		snip := ConfigSnippet{
			vars:         &nc.Vars,
			templateName: tn,
			TargetNode:   node,
			source:       "node",
		}

		err := snip.Render()
		if err != nil {
			return nil, err
		}
		snips = append(snips, snip)
	}
	return snips, nil
}

func RenderLink(link *clab.Link) ([]ConfigSnippet, error) {
	// Link labels/values are different on node A & B
	vars := make(map[string][]string)

	ncA := GetNodeConfigFromLabels(link.A.Node.Labels)
	ncB := GetNodeConfigFromLabels(link.B.Node.Labels)
	linkVars := link.Labels

	// Link IPs
	ipA, ipB, err := linkIPfromSystemIP(link)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", link, err)
	}
	vars["ip"] = []string{ipA.String(), ipB.String()}
	vars[systemIP] = []string{ncA.Vars[systemIP], ncB.Vars[systemIP]}

	// Split all fields with a comma...
	for k, v := range linkVars {
		r := strings.Split(v, ",")
		switch len(r) {
		case 1, 2:
			vars[k] = r
		default:
			log.Warnf("%s: %s contains %d elements, should be 1 or 2: %s", link.String(), k, len(r), v)
		}
	}

	// Set default Link/Interface Names
	if _, ok := vars["name"]; !ok {
		linkNr := linkVars["linkNr"]
		if len(linkNr) > 0 {
			linkNr = "_" + linkNr
		}
		vars["name"] = []string{fmt.Sprintf("to_%s%s", link.B.Node.ShortName, linkNr),
			fmt.Sprintf("to_%s%s", link.A.Node.ShortName, linkNr)}
	}

	snips := []ConfigSnippet{}

	for li := 0; li < 2; li++ {
		// Current Node
		curNode := link.A.Node
		if li == 1 {
			curNode = link.B.Node
		}
		// Current Vars
		curVars := make(map[string]string)
		for k, v := range vars {
			if len(v) == 1 {
				curVars[k] = strings.Trim(v[0], " \n\t")
			} else {
				curVars[k] = strings.Trim(v[li], " \n\t")
				curVars[k+"_far"] = strings.Trim(v[(li+1)%2], " \n\t")
			}
		}

		curNodeC := GetNodeConfigFromLabels(curNode.Labels)

		for _, tn := range curNodeC.Templates {
			snip := ConfigSnippet{
				vars:         &curVars,
				templateName: fmt.Sprintf("%s-link.tmpl", tn),
				TargetNode:   curNode,
				source:       link.String(),
			}
			err := snip.Render()
			//res, err := RenderTemplate(kind, tn, curVars, curNode, link.String())
			if err != nil {
				return nil, fmt.Errorf("render %s on %s (%s): %s", link, curNode.LongName, curNode.Kind, err)
			}
			snips = append(snips, snip)
		}
	}
	return snips, nil
}

// Implement stringer for conf snippet
func (c *ConfigSnippet) String() string {
	s := fmt.Sprintf("%s %s using %s/%s", c.TargetNode.ShortName, c.source, c.TargetNode.Kind, c.templateName)
	if c.Data != nil {
		s += fmt.Sprintf(" (%d lines)", bytes.Count(c.Data, []byte("\n"))+1)
	}
	return s
}

// Return the buffer as strings
func (c *ConfigSnippet) Lines() []string {
	return strings.Split(string(c.Data), "\n")
}

// Print the configSnippet
func (c *ConfigSnippet) Print(printLines int) {
	vars := []byte{}
	if log.IsLevelEnabled(log.DebugLevel) {
		vars, _ = json.MarshalIndent(c.vars, "", "  ")
	}

	s := ""
	if printLines > 0 {
		cl := strings.SplitN(string(c.Data), "\n", printLines+1)
		if len(cl) > printLines {
			cl[printLines] = "..."
		}
		s = "\n  | "
		s += strings.Join(cl, s)
	}

	log.Infof("%s %s%s\n", c.String(), vars, s)
}
