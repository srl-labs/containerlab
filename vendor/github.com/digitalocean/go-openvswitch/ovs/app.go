// Copyright 2017 DigitalOcean.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"fmt"
	"strings"
)

// AppService runs commands that are available from ovs-appctl
type AppService struct {
	c *Client
}

// ProtoTrace runs ovs-appctl ofproto/trace on the given bridge and match flow
// with the possibility to pass extra parameters like `--ct-next` and returns a *ProtoTrace.
// Also returns err if there is any error parsing the output from ovs-appctl ofproto/trace.
func (a *AppService) ProtoTrace(bridge string, protocol Protocol, matches []Match, params ...string) (*ProtoTrace, error) {
	matchFlows := []string{}
	if protocol != "" {
		matchFlows = append(matchFlows, string(protocol))
	}

	for _, match := range matches {
		matchFlow, err := match.MarshalText()
		if err != nil {
			return nil, err
		}

		matchFlows = append(matchFlows, string(matchFlow))
	}

	matchArg := strings.Join(matchFlows, ",")
	args := []string{"ofproto/trace", bridge, matchArg}
	args = append(args, params...)
	out, err := a.exec(args...)
	if err != nil {
		return nil, err
	}

	pt := &ProtoTrace{
		CommandStr: fmt.Sprintf("ovs-appctl %s", strings.Join(args, " ")),
	}
	err = pt.UnmarshalText(out)
	if err != nil {
		return nil, err
	}

	return pt, nil
}

// exec executes 'ovs-appctl' + args passed in
func (a *AppService) exec(args ...string) ([]byte, error) {
	return a.c.exec("ovs-appctl", args...)
}
