package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

type getConfig struct {
	XMLName  xml.Name     `xml:"get-config"`
	Source   *sourceT     `xml:""`
	Filter   *filterT     `xml:""`
	Defaults *defaultType `xml:""`
}

func (d *Driver) buildGetConfigElem(
	source, filter, filterType, defaultType string,
) (*message, error) {
	filterElem, err := d.buildFilterElem(filter, filterType)
	if err != nil {
		return nil, err
	}

	defaultsElem, err := d.buildDefaultsElem(defaultType)
	if err != nil {
		return nil, err
	}

	getConfigElem := &getConfig{
		XMLName:  xml.Name{},
		Source:   d.buildSourceElem(source),
		Filter:   filterElem,
		Defaults: defaultsElem,
	}

	netconfInput := d.buildPayload(getConfigElem)

	return netconfInput, nil
}

// GetConfig executes a get-config RPC against the NETCONF server.
func (d *Driver) GetConfig(source string, opts ...util.Option) (*response.NetconfResponse, error) {
	d.Logger.Infof("GetConfig RPC requested, source '%s'", source)

	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	m, err := d.buildGetConfigElem(source, op.Filter, op.FilterType, op.DefaultType)
	if err != nil {
		return nil, err
	}

	return d.sendRPC(m, op)
}
