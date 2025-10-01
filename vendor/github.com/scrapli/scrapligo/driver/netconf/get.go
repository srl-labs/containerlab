package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

type get struct {
	XMLName xml.Name `xml:"get"`
	Source  *sourceT `xml:""`
	Filter  *filterT `xml:""`
}

func (d *Driver) buildGetElem(
	filter, filterType string,
) (*message, error) {
	filterElem, err := d.buildFilterElem(filter, filterType)
	if err != nil {
		return nil, err
	}

	getElem := &get{
		XMLName: xml.Name{},
		Filter:  filterElem,
	}

	netconfInput := d.buildPayload(getElem)

	return netconfInput, nil
}

// Get executes a get RPC against the NETCONF server.
func (d *Driver) Get(filter string, opts ...util.Option) (*response.NetconfResponse, error) {
	d.Logger.Info("Get RPC requested")

	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	m, err := d.buildGetElem(filter, op.FilterType)
	if err != nil {
		return nil, err
	}

	return d.sendRPC(m, op)
}
