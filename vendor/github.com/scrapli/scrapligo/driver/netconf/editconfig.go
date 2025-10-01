package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
)

type editConfig struct {
	XMLName xml.Name `xml:"edit-config"`
	Target  *targetT `xml:""`
	Payload string   `xml:",innerxml"`
}

func (d *Driver) buildEditConfigElem(
	target, config string,
) *message {
	editConfigElem := &editConfig{
		XMLName: xml.Name{},
		Target:  d.buildTargetElem(target),
		Payload: config,
	}

	netconfInput := d.buildPayload(editConfigElem)

	return netconfInput
}

// EditConfig executes the edit-config RPC pushing the provided config against the target datastore.
func (d *Driver) EditConfig(target, config string) (*response.NetconfResponse, error) {
	d.Logger.Infof("EditConfig RPC requested, target '%s'", target)

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildEditConfigElem(target, config), op)
}
