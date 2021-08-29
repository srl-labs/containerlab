package utils

import (
	"time"

	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/driver/core"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/transport"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/srlinux-scrapli"
)

var (
	// map of commands per platform which start a CLI app
	NetworkOSCLICmd = map[string]string{
		"arista_eos":    "Cli",
		"nokia_srlinux": "sr_cli",
	}
)

// SpawnCLIviaExec spawns a CLI session over container runtime exec function
// end ensures the CLI is available to be used for sending commands over
func SpawnCLIviaExec(platform, contName string) (*network.Driver, error) {
	var d *network.Driver
	var err error

	switch platform {
	case "nokia_srlinux":
		d, err = srlinux.NewSRLinuxDriver(
			contName,
			base.WithAuthBypass(true),
			// disable transport timeout
			base.WithTimeoutTransport(0),
		)
	default:
		d, err = core.NewCoreDriver(
			contName,
			platform,
			base.WithAuthBypass(true),
			base.WithTimeoutTransport(0),
		)
	}

	if err != nil {
		log.Errorf("failed to create driver for device %s; error: %+v\n", err, contName)
		return nil, err
	}

	// TODO: implement for ctr (containerd)
	execCmd := "docker"
	openCmd := []string{"exec", "-it"}

	t, _ := d.Transport.(*transport.System)
	t.ExecCmd = execCmd
	t.OpenCmd = append(openCmd, contName, NetworkOSCLICmd[platform])

	transportReady := false
	for !transportReady {
		if err := d.Open(); err != nil {
			log.Debugf("%s - Cli not ready (%s) - waiting.", contName, err)
			time.Sleep(time.Second * 2)
			continue
		}
		transportReady = true
		log.Debugf("%s - Cli ready.", contName)
	}

	return d, err
}
