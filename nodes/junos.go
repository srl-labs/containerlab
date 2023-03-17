package nodes

import (
	"context"
	"fmt"
	"net"

	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/opoptions"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

func JunosAddLicense(ctx context.Context, n Node, defaultCredentials *Credentials, name, license string) error {
	log.Infof("adding license to node %s", name)
	// get container infos
	cnts, err := n.GetContainers(ctx)
	if err != nil {
		return err
	}

	// figure out if we have an ipv4 or an v6 available
	var mgmtIpPrefix string
	if mgmtIpPrefix = cnts[0].GetContainerIPv4(); mgmtIpPrefix == "N/A" {
		mgmtIpPrefix = cnts[0].GetContainerIPv6()
	}

	// if no valid v4 and v6 ip is present return err
	ip, _, err := net.ParseCIDR(mgmtIpPrefix)
	if err != nil {
		return err
	}

	// extract the ip as string
	mgmtIp := ip.String()

	// reading license file content
	bdata, err := utils.ReadFileContent(license)
	if err != nil {
		return err
	}

	// applying license to node
	err = junosAddLicenseNetconf(
		mgmtIp,
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		string(bdata),
	)
	if err != nil {
		return err
	}

	return nil
}

// junosAddLicenseNetconf utilizes scrapligo to push the license via netconf to the node
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
