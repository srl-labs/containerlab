// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/hellt/envsubst"

	"github.com/charmbracelet/log"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const (
	varFileSuffix = "_vars"
)

// ExportRenderedTopology controls whether the rendered topology YAML is saved to disk.
var ExportRenderedTopology string

// LoadTopologyFromFile loads a topology by the topo file path
// parses the topology file into c.Conf structure
// as well as populates the TopoFile structure with the topology file related information.
func (c *CLab) LoadTopologyFromFile(topo string, varsFiles []string) error {
	var err error

	c.TopoPaths, err = clabtypes.NewTopoPaths(topo, varsFiles)
	if err != nil {
		return err
	}

	// load the topology file/template
	topologyTemplate, err := template.New(c.TopoPaths.TopologyFilenameBase()).
		Funcs(clabutils.CreateFuncs()).
		ParseFiles(c.TopoPaths.TopologyFilenameAbsPath())
	if err != nil {
		return err
	}

	// if existing, load subtemplates that can be included in the topology file
	fsys := os.DirFS(c.TopoPaths.TopologyFileDir())
	subtemplates, err := fs.Glob(fsys, "clab_templates/*.gotmpl")
	if err != nil {
		return err
	}
	if len(subtemplates) != 0 {
		log.Debugf("found %d subtemplates, parsing...", len(subtemplates))
		upd, err := topologyTemplate.ParseFS(fsys, subtemplates...)
		if err != nil {
			return err
		}
		topologyTemplate = upd
	}
	log.Debugf("loading template variables...")

	// read template variables
	templateVars, err := readTemplateVariables(c.TopoPaths, varsFiles)
	if err != nil {
		return err
	}

	log.Debugf("template variables: %v", templateVars)

	buf := new(bytes.Buffer)

	err = topologyTemplate.Execute(buf, templateVars)
	if err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	log.Debugf("topology:\n%s\n", buf.String())

	// expand env vars if any
	// do not replace vars initialized with defaults
	// and do not replace vars that are not set
	yamlFile, err := envsubst.BytesRestrictedNoReplace(buf.Bytes(), false, false, true, true)
	if err != nil {
		return err
	}

	// save the rendered topology to disk if requested
	if ExportRenderedTopology != "" {
		if err := os.WriteFile(ExportRenderedTopology, yamlFile, 0644); err != nil {
			return fmt.Errorf(
				"failed to save rendered topology to %s: %w",
				ExportRenderedTopology,
				err,
			)
		}
		log.Infof("Rendered topology saved to %s", ExportRenderedTopology)
	}

	err = yaml.UnmarshalStrict(yamlFile, c.Config)
	if err != nil {
		return fmt.Errorf(
			"%w\nConsult with release notes to see if any fields were changed/removed", err,
		)
	}

	c.Config.Topology.ImportEnvs()

	return nil
}

func mergeTemplateVariables(dst, src any) any {
	dstMap, dstOK := dst.(map[string]any)
	srcMap, srcOK := src.(map[string]any)

	if !dstOK || !srcOK {
		// For non-maps, src overrides dst
		return src
	}

	for key, srcVal := range srcMap {
		if dstVal, exists := dstMap[key]; exists {
			dstMap[key] = mergeTemplateVariables(dstVal, srcVal)
		} else {
			dstMap[key] = srcVal
		}
	}

	return dstMap
}

func findVarsFiles(paths *clabtypes.TopoPaths) ([]string, error) {
	topo_dir := paths.TopologyFileDir()
	vars_search_glob := fmt.Sprintf("%s%s.*", paths.TopologyFilenameWithoutExt(), varFileSuffix)
	// e.g. lab_a.clab_vars.*

	// this will find both lab_a.clab_vars.yml, as well as lab_a.clab_vars.additions.yml;
	// their values will be merged in alphabetical order
	fsys := os.DirFS(topo_dir)

	valid_exts := []string{".yaml", ".yml", ".json"}

	result := []string{}
	candidates, err := fs.Glob(fsys, vars_search_glob)
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		candidate_ext := filepath.Ext(candidate)
		if slices.Contains(valid_exts, candidate_ext) {
			result = append(result, filepath.Join(topo_dir, candidate))
		}
	}

	return result, nil
}

func readTemplateVariables(paths *clabtypes.TopoPaths, varsFiles []string) (any, error) {
	if len(varsFiles) == 0 {
		log.Debug("searching for template vars files")
		foundFiles, err := findVarsFiles(paths)
		if err != nil {
			return nil, err
		}

		if len(foundFiles) == 0 {
			// no var file found, assume the topology is not a template
			// or a template that doesn't require external variables
			return nil, nil
		}
		varsFiles = foundFiles
	}

	log.Debug("template vars", "files", varsFiles)

	templateVars := make(map[string]any)
	// read all requested var files, and merge their contents into one:
	for _, varsFile := range varsFiles {
		// skip empty vars file names
		if len(varsFile) == 0 {
			continue
		}

		data, err := os.ReadFile(varsFile)
		if err != nil {
			return nil, err
		}

		var parsedVars map[string]any
		err = yaml.Unmarshal(data, &parsedVars)
		if err != nil {
			return nil, fmt.Errorf("variables file '%s': %w", filepath.Base(varsFile), err)
		}
		mergeTemplateVariables(templateVars, parsedVars)
	}

	return templateVars, nil
}
