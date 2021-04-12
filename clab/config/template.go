package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab"
)

type labelMap map[string]string
type ConfigSnippet struct {
	TargetNode           *clab.Node
	templateName, source string
	// All the labels used to render the template
	templateLabels *labelMap
	// Lines of config
	Config []string
}

// internal template cache
var templates map[string]*template.Template

func LoadTemplate(kind string, templatePath string) error {
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

func RenderTemplate(kind, name string, labels labelMap) (*ConfigSnippet, error) {
	t := templates[kind]

	buf := new(bytes.Buffer)

	err := t.ExecuteTemplate(buf, name, labels)
	if err != nil {
		log.Errorf("could not render template %s", err)
		b, _ := json.MarshalIndent(labels, "", "  ")
		log.Debugf("%s\n", b)
		return nil, err
	}

	var res []string
	s := bufio.NewScanner(buf)
	for s.Scan() {
		res = append(res, s.Text())
	}

	return &ConfigSnippet{
		templateLabels: &labels,
		templateName:   name,
		Config:         res,
	}, nil
}

func RenderNode(node *clab.Node) (*ConfigSnippet, error) {
	kind := node.Labels["clab-node-kind"]
	log.Debugf("render node %s [%s]\n", node.LongName, kind)

	res, err := RenderTemplate(kind, "base-node.tmpl", node.Labels)
	if err != nil {
		return nil, fmt.Errorf("render node %s [%s]: %s", node.LongName, kind, err)
	}
	res.source = "node"
	res.TargetNode = node
	return res, nil
}

func RenderLink(link *clab.Link) (*ConfigSnippet, *ConfigSnippet, error) {
	// Link labels/values are different on node A & B
	l := make(map[string][]string)

	// Link IPs
	ipA, ipB, err := linkIPfromSystemIP(link)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", link, err)
	}
	l["ip"] = []string{ipA.String(), ipB.String()}
	l["systemip"] = []string{link.A.Node.Labels[systemIP], link.B.Node.Labels[systemIP]}

	// Split all fields with a comma...
	for k, v := range link.Labels {
		r := strings.Split(v, ",")
		switch len(r) {
		case 1:
		case 2:
			l[k] = r
		default:
			log.Warnf("%s: %s contains %d elements: %s", link, k, len(r), v)
		}
	}

	// Set default Link/Interface Names
	if _, ok := l["name"]; !ok {
		linkNr := link.Labels["linkNr"]
		if len(linkNr) > 0 {
			linkNr = "_" + linkNr
		}
		l["name"] = []string{fmt.Sprintf("to_%s%s", link.B.Node.ShortName, linkNr),
			fmt.Sprintf("to_%s%s", link.A.Node.ShortName, linkNr)}
	}

	log.Debugf("%s: %s\n", link, l)

	var res, resA *ConfigSnippet

	var curL labelMap
	var curN *clab.Node

	for li := 0; li < 2; li++ {
		if li == 0 {
			// set current node as A
			curN = link.A.Node
			curL = make(labelMap)
			for k, v := range l {
				curL[k] = v[0]
				if len(v) > 1 {
					curL[k+"_far"] = v[1]
				}
			}
		} else {
			curN = link.B.Node
			curL = make(labelMap)
			for k, v := range l {
				if len(v) == 1 {
					curL[k] = v[0]
				} else {
					curL[k] = v[1]
					curL[k+"_far"] = v[0]
				}
			}
		}
		// Render the links
		kind := curN.Labels["clab-node-kind"]
		log.Debugf("render %s on %s (%s) - %s", link, curN.LongName, kind, curL)
		res, err = RenderTemplate(kind, "base-link.tmpl", curL)
		if err != nil {
			return nil, nil, fmt.Errorf("render %s on %s (%s): %s", link, curN.LongName, kind, err)
		}
		res.source = link.String()
		res.TargetNode = curN
		if li == 0 {
			resA = res
		}
	}
	return resA, res, nil
}

// Implement stringer for conf snippet
func (c *ConfigSnippet) String() string {
	return fmt.Sprintf("%s: %s %d lines of config", c.TargetNode.LongName, c.source, len(c.Config))
}

var funcMap = map[string]interface{}{
	"require": func(val interface{}) (interface{}, error) {
		if val == nil {
			return nil, errors.New("required value not set")
		}
		return val, nil
	},
	"ip": func(val interface{}) (interface{}, error) {
		s := fmt.Sprintf("%v", val)
		a := strings.Split(s, "/")
		return a[0], nil
	},
	"ipmask": func(val interface{}) (interface{}, error) {
		s := fmt.Sprintf("%v", val)
		a := strings.Split(s, "/")
		return a[1], nil
	},
	"default": func(val interface{}, def interface{}) (interface{}, error) {
		if val == nil {
			return def, nil
		}
		return val, nil
	},
	"contains": func(str interface{}, substr interface{}) (interface{}, error) {
		return strings.Contains(fmt.Sprintf("%v", str), fmt.Sprintf("%v", substr)), nil
	},
	"slice": func(val interface{}, start interface{}, end interface{}) (interface{}, error) {
		v := fmt.Sprintf("%v", val)
		s := int(start.(int))
		e := int(end.(int))
		if s < 0 {
			s += len(v)
		}
		if e < 0 {
			e += len(v)
		}
		return v[s:e], nil
	},
}
