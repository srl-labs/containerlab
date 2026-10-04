package utils

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
	scrapligooptions "github.com/scrapli/scrapligo/v2/options"
)

var (
	// map of commands per platform which start a CLI app.
	NetworkOSCLICmd = map[string][]string{
		"arista_eos":    {"Cli"},
		"juniper_junos": {"cli"},
		"nokia_srlinux": {"sr_cli"},
		"vyos_vyos":     {"su", "-", "admin"},
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
func SpawnCLIviaExec(platformName, contName, runtimeName string) (*scrapligocli.Cli, error) {
	// the bin transport open args are fully overridden, since we do not use
	// ssh to reach the device, but rather the container runtime exec command.
	openArgs := strings.Join(
		slices.Concat(
			strings.Split(CLIExecCommand[runtimeName]["open"], " "),
			[]string{contName},
			NetworkOSCLICmd[platformName],
		),
		" ",
	)

	opts := []scrapligooptions.Option{
		scrapligooptions.WithDefinitionFileOrName(platformName),
		scrapligooptions.WithTransportBin(),
		scrapligooptions.WithBinTransportBinOverride(CLIExecCommand[runtimeName]["exec"]),
		scrapligooptions.WithBinTransportOverrideArgs(openArgs),
		// there is no authentication to exec, so make sure scrapli does not
		// try to authenticate in the session.
		scrapligooptions.WithBypassInSessionAuth(),
	}

	// jack up TermWidth, since we use `docker exec` to enter certificate and key strings
	// and these are lengthy
	if strings.HasPrefix(platformName, "nokia_srl") {
		opts = append(opts, scrapligooptions.WithTermWidth(5000))
	}

	c, err := scrapligocli.NewCli(contName, opts...)
	if err != nil {
		log.Errorf("failed to fetch platform instance for device %s; error: %+v\n", err, contName)
		return nil, err
	}

	for {
		if _, err := c.Open(context.Background()); err != nil {
			log.Debugf("%s - Cli not ready (%s) - waiting.", contName, err)
			time.Sleep(time.Second * 2)
			continue
		}

		log.Debugf("%s - Cli ready.", contName)

		return c, nil
	}
}
