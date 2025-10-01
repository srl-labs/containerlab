package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
)

type copyConfig struct {
	XMLName xml.Name `xml:"copy-config"`
	Target  *targetT `xml:""`
	Source  *sourceT `xml:""`
}

func (d *Driver) buildCopyConfigElem(
	source,
	target string,
) *message {
	copyConfigElem := &copyConfig{
		XMLName: xml.Name{},
		Target:  d.buildTargetElem(target),
		Source:  d.buildSourceElem(source),
	}

	netconfInput := d.buildPayload(copyConfigElem)

	return netconfInput
}

// CopyConfig executes a copy config RPC against the NETCONF server copying the source to the
// target datastore.
func (d *Driver) CopyConfig(source, target string) (*response.NetconfResponse, error) {
	d.Logger.Infof("CopyConfig RPC requested, source '%s'/target '%s'", source, target)

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildCopyConfigElem(source, target), op)
}
