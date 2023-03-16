package nodes

import (
	"context"
	"fmt"

	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/opoptions"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

func JunosAddLicense(ctx context.Context, n Node, defaultCredentials *Credentials, name, license string) error {
	// get container infos
	cnts, err := n.GetContainers(ctx)
	if err != nil {
		return err
	}

	// figure out if we have an ipv4 or an v6 available
	var mgmtAddr string
	if mgmtAddr = cnts[0].GetContainerIPv4(); mgmtAddr == "N/A" {
		mgmtAddr = cnts[0].GetContainerIPv6()
	}

	// if any ip (v4 or v6) is present add the license
	if mgmtAddr != "N/A" {
		log.Infof("adding license to node %s", name)
		bdata, err := utils.ReadFileContent(license)
		if err != nil {
			return err
		}
		err = junosAddLicenseNetconf(
			mgmtAddr,
			defaultCredentials.GetUsername(),
			defaultCredentials.GetPassword(),
			string(bdata),
		)
		if err != nil {
			return err
		}
	} else {
		log.Errorf("unable to add license to node %s. no mgmt ip available", name)
	}
	return nil
}

func junosAddLicenseNetconf(addr, username, password, licenseData string) error {
	opts := []util.Option{
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
		options.WithTransportType(transport.StandardTransport),
		options.WithPort(830),
	}

	d, err := netconf.NewDriver(
		addr,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("could not create netconf driver for %s: %+v", addr, err)
	}

	err = d.Open()
	if err != nil {
		return fmt.Errorf("failed to open netconf driver for %s: %+v", addr, err)
	}
	defer d.Close()

	resp, err := d.RPC(opoptions.WithFilter(fmt.Sprintf("<rpc><request-license-add><key-data>%s</key-data></request-license-add></rpc>", licenseData)))
	if err != nil {
		return err
	}
	if resp.Failed != nil {
		return resp.Failed
	}

	return nil
}
