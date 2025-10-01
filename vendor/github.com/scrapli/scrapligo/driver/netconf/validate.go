package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
)

type validate struct {
	XMLName xml.Name `xml:"validate"`
	Source  *sourceT `xml:""`
}

func (d *Driver) buildValidateElem(source string) *message {
	validateElem := &validate{
		XMLName: xml.Name{},
		Source:  d.buildSourceElem(source),
	}

	netconfInput := d.buildPayload(validateElem)

	return netconfInput
}

// Validate executes validate RPC for the source datastore against the NETCONF server.
func (d *Driver) Validate(source string) (*response.NetconfResponse, error) {
	d.Logger.Infof("Validate RPC requested, source '%s'", source)

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildValidateElem(source), op)
}
