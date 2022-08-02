// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	labels      []string
	execFormat  string
	execCommand string
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:     "exec",
	Short:   "execute a command on one or multiple containers",
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && topo == "" {
			return errors.New("provide either lab name (--name) or topology file path (--topo)")
		}

		if execCommand == "" {
			return errors.New("provide command to execute")
		}

		switch execFormat {
		case "json", "plain":
			// expected values, go on
		default:
			log.Error("format is expected to be either json or plain")
		}
		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo, varsFile),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
		}
		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		if name == "" {
			name = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		filters := []*types.GenericFilter{{FilterType: "label", Match: name, Field: "containerlab", Operator: "="}}
		filters = append(filters, types.FilterFromLabelStrings(labels)...)
		containers, err := c.ListContainers(ctx, filters)
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			return errors.New("no containers found")
		}

		jsonResult := make(map[string]map[string]map[string]interface{})
		for _, cont := range containers {
			if cont.State != "running" {
				continue
			}
			if len(cont.Names) == 0 {
				continue
			}
			nodeRuntime, err := c.GetNodeRuntime(strings.TrimPrefix(cont.Names[0], "/"))
			if err != nil {
				return err
			}

			contName := strings.TrimLeft(cont.Names[0], "/")
			if jsonResult[contName], err = execCmds(
				ctx, cont, nodeRuntime, []string{execCommand}, execFormat,
			); err != nil {
				return err
			}
		}
		if execFormat == "json" {
			result, err := json.Marshal(jsonResult)
			if err != nil {
				log.Errorf("Issue converting to json %v", err)
			}
			fmt.Println(string(result))
		}
		return err
	},
}

func execCmds(
	ctx context.Context,
	cont types.GenericContainer,
	runtime runtime.ContainerRuntime,
	cmds []string,
	format string,
) (result map[string]map[string]interface{}, err error) {
	var doc interface{}

	result = make(map[string]map[string]interface{})
	for _, cmd := range cmds {
		c, err := shlex.Split(cmd)
		if err != nil {
			return nil, err
		}

		stdout, stderr, err := runtime.Exec(ctx, cont.ID, c)
		if err != nil {
			log.Errorf("%s: failed to execute cmd: %v", cont.Names, err)
			return nil, nil
		}

		switch format {
		case "json":
			result[cmd] = make(map[string]interface{})
			if json.Unmarshal([]byte(stdout), &doc) == nil {
				result[cmd]["stdout"] = doc
			} else {
				result[cmd]["stdout"] = string(stdout)
			}
			result[cmd]["stderr"] = string(stderr)
		case "plain", "table":
			contName := strings.TrimLeft(cont.Names[0], "/")
			if len(stdout) > 0 {
				log.Infof("Executed command '%s' on %s. stdout:\n%s", cmd, contName, string(stdout))
			}
			if len(stderr) > 0 {
				log.Infof("Executed command '%s' on %s. stderr:\n%s", cmd, contName, string(stderr))
			}
		}
	}

	return result, nil
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVarP(&execCommand, "cmd", "", "", "command to execute")
	execCmd.Flags().StringSliceVarP(&labels, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
