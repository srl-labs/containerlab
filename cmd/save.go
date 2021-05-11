package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/Juniper/go-netconf/netconf"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/types"
)

var saveCommand = map[string][]string{
	"srl":  {"sr_cli", "-d", "tools", "system", "configuration", "generate-checkpoint"},
	"ceos": {"Cli", "-p", "15", "-c", "copy running flash:conf-saved.conf"},
	"crpd": {"cli", "show", "conf"},
}

// saveCmd represents the save command
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.srlinux.dev/cmd/save/ documentation to see the exact command used per node's kind`,
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && topo == "" {
			return fmt.Errorf("provide topology file path  with --topo flag")
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithRuntime(rt, debug, timeout, graceful),
		}
		c := clab.NewContainerLab(opts...)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		containers, err := c.Runtime.ListContainers(ctx, []string{"containerlab=" + c.Config.Name})
		if err != nil {
			return fmt.Errorf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			return fmt.Errorf("no containers found")
		}

		var wg sync.WaitGroup
		wg.Add(len(containers))
		for _, cont := range containers {
			go func(cont types.GenericContainer) {
				defer wg.Done()
				kind := cont.Labels["clab-node-kind"]

				switch kind {
				case "vr-sros",
					"vr-vmx":
					netconfSave(cont)
					return
				}

				// skip saving if we have no command map
				if _, ok := saveCommand[kind]; !ok {
					return
				}
				stdout, stderr, err := c.Runtime.Exec(ctx, cont.ID, saveCommand[kind])
				if err != nil {
					log.Errorf("%s: failed to execute cmd: %v", cont.Names, err)

				}
				if len(stderr) > 0 {
					log.Infof("%s errors: %s", strings.TrimLeft(cont.Names[0], "/"), string(stderr))
				}
				switch {
				// for srl kinds print the full stdout
				case kind == "srl":
					if len(stdout) > 0 {
						confPath := cont.Labels["clab-node-dir"] + "/config/checkpoint/checkpoint-0.json"
						log.Infof("saved SR Linux configuration from %s node to %s\noutput:\n%s", strings.TrimLeft(cont.Names[0], "/"), confPath, string(stdout))
					}

				case kind == "crpd":
					// path by which to save a config
					confPath := cont.Labels["clab-node-dir"] + "/config/conf-saved.conf"
					err := ioutil.WriteFile(confPath, stdout, 0777)
					if err != nil {
						log.Errorf("failed to write config by %s path from %s container: %v", confPath, strings.TrimLeft(cont.Names[0], "/"), err)
					}
					log.Infof("saved cRPD configuration from %s node to %s", strings.TrimLeft(cont.Names[0], "/"), confPath)

				case kind == "ceos":
					// path by which a config was saved
					confPath := cont.Labels["clab-node-dir"] + "/flash/conf-saved.conf"
					log.Infof("saved cEOS configuration from %s node to %s", strings.TrimLeft(cont.Names[0], "/"), confPath)
				}
			}(cont)
		}
		wg.Wait()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

func netconfSave(cont types.GenericContainer) {
	kind := cont.Labels["clab-node-kind"]
	config := netconf.SSHConfigPassword(clab.DefaultCredentials[kind][0],
		clab.DefaultCredentials[kind][1])

	host := strings.TrimLeft(cont.Names[0], "/")
	ncHost := host + ":830"

	s, err := netconf.DialSSH(ncHost, config)
	if err != nil {
		log.Errorf("%s: Could not connect SSH to %s %s", cont.Names[0], host, err)
		return
	}
	defer s.Close()

	save := `<copy-config><target><startup/></target><source><running/></source></copy-config>`

	_, err = s.Exec(netconf.RawMethod(save))
	if err != nil {
		log.Errorf("%s: Could not send Netconf save to %s %s", cont.Names[0], host, err)
		return
	}

	log.Infof("saved %s configuration from node %s\n", kind, host)
}
