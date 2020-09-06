package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// saveCmd represents the save command
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Run: func(cmd *cobra.Command, args []string) {
		if prefix == "" && topo == "" {
			fmt.Println("provide either lab prefix (--prefix) or topology file path (--topo)")
			return
		}
		c := clab.NewContainerLab(debug)
		err := c.Init()
		if err != nil {
			log.Fatal(err)
		}
		if prefix == "" {
			if err = c.GetTopology(&topo); err != nil {
				log.Fatal(err)
			}
			prefix = c.Conf.Prefix
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		filter := filters.NewArgs()
		filter.Add("label", fmt.Sprintf("containerlab=lab-%s", prefix))
		containers, err := c.DockerClient.ContainerList(ctx, types.ContainerListOptions{
			Filters: filter,
		})
		if err != nil {
			log.Fatalf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			log.Println("no containers found")
			return
		}
		var saveCmd []string
		for _, cont := range containers {
			log.Debugf("container: %+v", cont)
			if k, ok := cont.Labels["kind"]; ok {
				switch k {
				case "srl":
					saveCmd = []string{"sr_cli", "-d", "tools", "system", "configuration", "generate-checkpoint"}
				case "ceos":
					//TODO
				default:
					continue
				}
			}
			id, err := c.DockerClient.ContainerExecCreate(ctx, cont.ID, types.ExecConfig{
				User:         "root",
				AttachStderr: true,
				AttachStdout: true,
				Cmd:          saveCmd,
			})
			if err != nil {
				log.Errorf("failed to create exec in container %s: %v", cont.Names, err)
			}
			log.Debugf("%s exec created %v", cont.Names, id)
			rsp, err := c.DockerClient.ContainerExecAttach(ctx, id.ID, types.ExecConfig{
				User:         "root",
				AttachStderr: true,
				AttachStdout: true,
				Cmd:          saveCmd,
			})
			if err != nil {
				log.Errorf("failed exec in container %s: %v", cont.Names, err)
			}
			defer rsp.Close()
			log.Debugf("%s exec attached %v", cont.Names, id)

			var outBuf, errBuf bytes.Buffer
			outputDone := make(chan error)

			go func() {
				_, err = stdcopy.StdCopy(&outBuf, &errBuf, rsp.Reader)
				outputDone <- err
			}()

			select {
			case err := <-outputDone:
				if err != nil {
					log.Errorf("%s: command exec error: %v", cont.Names, err)
					log.Errorf("%s: stdout: %s", cont.Names, outBuf.String())
					log.Errorf("%s: stderr: %s", cont.Names, errBuf.String())
					continue
				}
			case <-ctx.Done():
				return
			}
			stdout, err := ioutil.ReadAll(&outBuf)
			if err != nil {
				log.Errorf("%s failed to read stdout buffer: %v", cont.Names, err)
			}
			stderr, err := ioutil.ReadAll(&errBuf)
			if err != nil {
				log.Errorf("%s failed to read stderr buffer: %v", cont.Names, err)
			}
			if len(stdout) > 0 {
				log.Infof("%s: stdout: %s", cont.Names, string(stdout))
			}
			if len(stderr) > 0 {
				log.Infof("%s: stderr: %s", cont.Names, string(stderr))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
	saveCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
	saveCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "lab name prefix")
}
