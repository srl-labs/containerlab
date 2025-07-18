package utils

import (
	"slices"
	"strings"
	"time"

	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/util"

	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/options"
)

var (
	// map of commands per platform which start a CLI app.
	NetworkOSCLICmd = map[string][]string{
		"arista_eos":    {"Cli"},
		"nokia_srlinux": {"sr_cli"},
		"vyatta_vyos":   {"su", "-", "admin"},
	}

	// map of the cli exec command and its argument per runtime
	// which is used to spawn CLI session.
	CLIExecCommand = map[string]map[string]string{
		"docker": {
			"exec": "docker",
			"open": "exec -it",
		},
		"podman": {
			"exec": "podman",
			"open": "exec -it",
		},
	}
)

// SpawnCLIviaExec spawns a CLI session over container runtime exec function
// end ensures the CLI is available to be used for sending commands over.
func SpawnCLIviaExec(platformName, contName, runtime string) (*network.Driver, error) {
	opts := []util.Option{
		options.WithAuthBypass(),
		options.WithSystemTransportOpenBin(CLIExecCommand[runtime]["exec"]),
		options.WithSystemTransportOpenArgsOverride(
			slices.Concat(
				strings.Split(CLIExecCommand[runtime]["open"], " "),
				[]string{contName},
				NetworkOSCLICmd[platformName],
			),
		),
	}

	// scrapligo v1.0.0 changed to just "nokia_srl" rather than "nokia_srlinux"
	if strings.HasPrefix(platformName, "nokia_srl") {
		// jack up TermWidth, since we use `docker exec` to enter certificate and key strings
		// and these are lengthy
		opts = append(opts, options.WithTermWidth(5000))
	}

	var p *platform.Platform

	var d *network.Driver

	var err error

	for {
		p, err = platform.NewPlatform(
			platformName,
			contName,
			opts...,
		)
		if err != nil {
			log.Errorf("failed to fetch platform instance for device %s; error: %+v\n", err, contName)
			return nil, err
		}

		d, err = p.GetNetworkDriver()
		if err != nil {
			log.Errorf("failed to create driver for device %s; error: %+v\n", err, contName)
			return nil, err
		}

		if err = d.Open(); err != nil {
			log.Debugf("%s - Cli not ready (%s) - waiting.", contName, err)
			time.Sleep(time.Second * 2)
			continue
		}

		log.Debugf("%s - Cli ready.", contName)

		return d, nil
	}
}
