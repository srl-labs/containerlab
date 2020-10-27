package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Short:   "destroy a lab",
	Aliases: []string{"des"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := clab.NewContainerLab(debug)
		err := c.Init(timeout)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err = c.GetTopology(&topo); err != nil {
			return err
		}

		// Parse topology information
		if err = c.ParseTopology(); err != nil {
			return err
		}

		containers, err := c.ListContainers(ctx, []string{fmt.Sprintf("containerlab=lab-%s", c.Conf.Prefix)})
		if err != nil {
			return fmt.Errorf("could not list containers: %v", err)
		}

		log.Infof("Destroying container lab: %s", topo)
		wg := new(sync.WaitGroup)
		wg.Add(len(containers))
		for _, cont := range containers {
			go func(cont types.Container) {
				defer wg.Done()
				name := cont.ID
				if len(cont.Names) > 0 {
					name = cont.Names[0]
				}
				log.Infof("Stopping container: %s", name)
				err = c.DeleteContainer(ctx, name, cont.ID, 30*time.Second)
				if err != nil {
					log.Errorf("could not remove container '%s': %v", name, err)
				}
			}(cont)
		}
		wg.Wait()
		err = deleteEntriesFromHostsFile(containers, c.Conf.DockerInfo.Bridge)
		if err != nil {
			return err
		}

		// delete container management bridge
		log.Info("Deleting docker bridge ...")
		if err = c.DeleteBridge(ctx); err != nil {
			log.Error(err)
		}
		// delete virtual wiring
		for _, link := range c.Links {
			if err = c.DeleteVirtualWiring(link); err != nil {
				log.Error(err)
			}
		}
		c.InitVirtualWiring()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

func deleteEntriesFromHostsFile(containers []types.Container, bridgeName string) error {
	if bridgeName == "" {
		return fmt.Errorf("missing bridge name")
	}
	f, err := os.OpenFile("/etc/hosts", os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data := hostsEntries(containers, bridgeName)
	remainingLines := make([][]byte, 0)
	reader := bufio.NewReader(f)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		found := false
		sLine := strings.Join(strings.Fields(string(line)), " ")
		for _, dl := range strings.Split(string(data), "\n") {
			sdl := strings.Join(strings.Fields(string(dl)), " ")
			if strings.Compare(sLine, sdl) == 0 {
				found = true
				break
			}
		}
		if !found {
			remainingLines = append(remainingLines, line)
		}
	}

	err = f.Truncate(0)
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}
	for _, l := range remainingLines {
		f.Write(l)
		f.Write([]byte("\n"))
	}
	return nil
}
