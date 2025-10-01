package network

import (
	"strings"
	"time"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

// SendConfig is a convenience wrapper around SendConfigs. This method accepts a config string which
// is split on new lines and sent as configLines to SendConfigs. The resulting
// response.MultiResponse is then collapsed into a single response.Response object.
func (d *Driver) SendConfig(config string, opts ...util.Option) (*response.Response, error) {
	configLines := strings.Split(config, "\n")

	m, err := d.SendConfigs(configLines, opts...)
	if err != nil {
		return nil, err
	}

	r := response.NewResponse(
		config,
		d.Transport.GetHost(),
		d.Transport.GetPort(),
		m.Responses[0].FailedWhenContains,
	)

	rOutputs := make([]string, len(m.Responses))
	for i, resp := range m.Responses {
		rOutputs[i] = resp.Result
	}

	r.StartTime = m.StartTime
	r.EndTime = time.Now()
	r.ElapsedTime = r.EndTime.Sub(r.StartTime).Seconds()
	r.Result = strings.Join(rOutputs, "\n")
	r.Failed = m.Failed

	return r, nil
}
