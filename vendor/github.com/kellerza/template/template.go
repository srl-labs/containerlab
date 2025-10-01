// MIT License, copyright (c) 2021 Johann Kellerman

// Package template implements a wrapper around the Go standard library
// test/template
//
// It includes helpers to render the template and several additional functions.
// Additional template functions are exposed as public functions, so it shows up
// in https://pkg.go.dev/github.com/kellerza/template#pkg-functions
package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	tmpl "text/template"

	log "github.com/sirupsen/logrus"
)

type TemplateOption func(*Template) error
type Template struct {
	T          *tmpl.Template
	searchPath []string
	names      map[string]string
}

// Get an instance of Template (text/template's Template with all functions added)
func New(name string, options ...TemplateOption) (*Template, error) {
	t := &Template{
		T:     tmpl.New(name).Funcs(Funcs),
		names: make(map[string]string),
	}
	for _, opt := range options {
		err := opt(t)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func ErrorOnMissingKey() TemplateOption {
	return func(t *Template) error {
		t.T.Option("missingkey=error")
		return nil
	}
}

// Add paths to search for templates
func SearchPath(paths ...string) TemplateOption {
	return func(t *Template) error {
		t.searchPath = paths
		return nil
	}
}

// Load a template. It will search backward through the SearchPath.
func (t *Template) load(name string) error {
	if t.T.Lookup(name) != nil {
		return nil
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("the template name should not include a path, use the search path: %s", name)
	}
	for i := len(t.searchPath) - 1; i >= 0; i-- {

		fn := filepath.Join(t.searchPath[i], name)
		_, err := t.T.ParseFiles(fn)
		if os.IsNotExist(err) { // try in next path
			log.Debugf("template not found: %s\n", fn)
			continue
		}
		if err != nil {
			return fmt.Errorf("could not load template %s: %s", fn, err)
		}
		log.Debugf("template loaded %d. %s %s\n", i, name, fn)
		t.names[name] = fn
		return nil
	}
	return fmt.Errorf("could not find template %s in search path", name)
}

func execute(tmpl *tmpl.Template, vars map[string]interface{}) (string, error) {
	varsP, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		varsP = []byte(fmt.Sprintf("%s", vars))
	}
	log.Debugf("execute template %s vars=%s\n", tmpl.Name(), varsP)
	var buf strings.Builder
	err = tmpl.Execute(&buf, vars)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Execute the template
func (t *Template) Execute(vars map[string]interface{}) (string, error) {
	return execute(t.T, vars)
}

// Execute a specific template
func (t *Template) ExecuteTemplate(name string, vars map[string]interface{}) (string, error) {
	err := t.load(name)
	if err != nil {
		return "", err
	}
	res, err := execute(t.T.Lookup(name), vars)
	if err != nil {
		return "", fmt.Errorf("could not render template %s: %s", t.names[name], err)
	}
	return res, nil
}
