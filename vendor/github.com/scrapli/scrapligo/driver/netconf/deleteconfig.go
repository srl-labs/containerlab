package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
)

type deleteConfig struct {
	XMLName xml.Name `xml:"delete-config"`
	Target  *targetT `xml:""`
}

func (d *Driver) buildDeleteConfigElem(
	target string,
) *message {
	deleteConfigElem := &deleteConfig{
		XMLName: xml.Name{},
		Target:  d.buildTargetElem(target),
	}

	netconfInput := d.buildPayload(deleteConfigElem)

	return netconfInput
}

// DeleteConfig executes the delete-config RPC against the NETCONF server deleting the target
// datastore.
func (d *Driver) DeleteConfig(target string) (*response.NetconfResponse, error) {
	d.Logger.Infof("DeleteConfig RPC requested, target '%s'", target)

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildDeleteConfigElem(target), op)
}
