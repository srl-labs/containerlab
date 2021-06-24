// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/types"
)

var (
	labels     []string
	execFormat string
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:     "exec",
	Short:   "execute a command on one or multiple containers",
	PreRunE: sudoCheck,
	Run: func(cmd *cobra.Command, args []string) {
		if name == "" && topo == "" {
			fmt.Println("provide either lab name (--name) or topology file path (--topo)")
			return
		}
		log.Debugf("raw command: %v", args)
		if len(args) == 0 {
			fmt.Println("provide command to execute")
			return
		}
		switch execFormat {
		case "json",
			"plain":
			// expected values, go on
		default:
			log.Error("format is expected to be either json or plain")
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithRuntime(rt, debug, timeout, graceful),
		}
		c := clab.NewContainerLab(opts...)

		if name == "" {
			name = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		filters := []*types.GenericFilter{{FilterType: "label", Match: name, Field: "containerlab", Operator: "="}}
		filters = append(filters, types.FilterFromLabelStrings(labels)...)
		containers, err := c.Runtime.ListContainers(ctx, filters)
		if err != nil {
			log.Fatalf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			log.Println("no containers found")
			return
		}
		cmds := make([]string, 0, len(args))
		for _, a := range args {
			cmds = append(cmds, strings.Split(a, " ")...)
		}
		jsonResult := make(map[string]map[string]interface{})

		for _, cont := range containers {
			var doc interface{}
			if cont.State != "running" {
				continue
			}
			stdout, stderr, err := c.Runtime.Exec(ctx, cont.ID, cmds)
			if err != nil {
				log.Errorf("%s: failed to execute cmd: %v", cont.Names, err)
				continue
			}
			contName := strings.TrimLeft(cont.Names[0], "/")
			switch execFormat {
			case "json":
				jsonResult[contName] = make(map[string]interface{})
				err := json.Unmarshal([]byte(stdout), &doc)
				if err == nil {
					jsonResult[contName]["stdout"] = doc
				} else {
					jsonResult[contName]["stdout"] = string(stdout)
				}
				jsonResult[contName]["stderr"] = string(stderr)
			case "plain":
				if len(stdout) > 0 {
					log.Infof("%s: stdout:\n%s", contName, string(stdout))
				}
				if len(stderr) > 0 {
					log.Infof("%s: stderr:\n%s", contName, string(stderr))
				}

			}
		}
		if execFormat == "json" {
			result, err := json.Marshal(jsonResult)
			if err != nil {
				log.Errorf("Issue converting to json %v", err)
			}
			fmt.Println(string(result))
		}
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringSliceVarP(&labels, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
