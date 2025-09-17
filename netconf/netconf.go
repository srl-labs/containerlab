// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package netconf contains netconf-based utility functions used in containerlab.
package netconf

import (
	"bytes"
	"fmt"
	"html"

	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	"github.com/scrapli/scrapligocfg"
)

// SaveRunningConfig saves the running config to the startup by means
// of invoking a netconf rpc <copy-config> from running to startup datastore
// this method is used on the network elements that can't perform configuration save via other means.
func SaveRunningConfig(addr, username, password, _ string) error {
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

	_, err = d.CopyConfig("running", "startup")
	if err != nil {
		return fmt.Errorf("%s: Could not send save config via Netconf: %+v", addr, err)
	}

	return nil
}

// GetConfig retrieves the running configuration and returns it as a string. It automatically picks the appropriate network driver for the provided Scrapli Platform.
func GetConfig(addr, username, password, scrapliPlatform string) (string, error) {
	p, err := platform.NewPlatform(
		scrapliPlatform,
		addr,
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
		options.WithTransportType(transport.StandardTransport),
		options.WithPort(22),
	)
	if err != nil {
		return "", fmt.Errorf("could not create or missing platform driver for %s: %+v", addr, err)
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		return "", fmt.Errorf("could not create generic driver for %s: %+v", addr, err)
	}

	err = d.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open generic driver for %s: %+v", addr, err)
	}
	defer d.Close()

	cfg, err := scrapligocfg.NewCfg(d, scrapliPlatform)
	if err != nil {
		return "", fmt.Errorf("failed to instantiate scrapligocfg for %s: %+v", addr, err)
	}

	err = cfg.Prepare()
	if err != nil {
		return "", fmt.Errorf("failed to prepare scraplicfg connection for %s: %+v", addr, err)
	}

	config, err := cfg.GetConfig("running")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve config via scraplicfg for %s: %+v", addr, err)
	}

	log.Debug("Retrieved node config via scraplicfg", "config", config.Result)

	return config.Result, nil
}

// Operation defines a NETCONF action to be executed against an established NETCONF driver.
type Operation func(*netconf.Driver) error

// MultiExec opens a NETCONF session to the provided address and executes the supplied operations
// sequentially. The driver is opened once and used across every operation, enabling scenarios that
// require multiple NETCONF calls within a single session (for example, chaining import actions prior
// to committing configuration changes).
func MultiExec(addr, username, password string, operations []Operation) error {
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

	for idx, operation := range operations {
		if err = operation(d); err != nil {
			return fmt.Errorf("netconf operation %d failed: %w", idx+1, err)
		}
	}

	return nil
}

// XMLBuilder provides an interface for building XML
type XMLBuilder struct {
	buf    bytes.Buffer
	indent int
	pretty bool
}

// NewXMLBuilder creates a new XML builder
func NewXMLBuilder() *XMLBuilder {
	return &XMLBuilder{pretty: false}
}

// SetPretty enables/disables pretty printing
func (b *XMLBuilder) SetPretty(pretty bool) *XMLBuilder {
	b.pretty = pretty
	return b
}

// StartElement starts a new XML element with optional attributes
func (b *XMLBuilder) StartElement(name string, attrs ...string) *XMLBuilder {
	if b.pretty && b.buf.Len() > 0 {
		b.buf.WriteString("\n")
		b.writeIndent()
	}

	b.buf.WriteString("<")
	b.buf.WriteString(name)

	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			b.buf.WriteString(" ")
			b.buf.WriteString(attrs[i])
			b.buf.WriteString(`="`)
			b.buf.WriteString(html.EscapeString(attrs[i+1]))
			b.buf.WriteString(`"`)
		}
	}

	b.buf.WriteString(">")
	b.indent++
	return b
}

// EndElement closes an XML element
func (b *XMLBuilder) EndElement(name string) *XMLBuilder {
	b.indent--
	if b.pretty {
		b.buf.WriteString("\n")
		b.writeIndent()
	}

	b.buf.WriteString("</")
	b.buf.WriteString(name)
	b.buf.WriteString(">")
	return b
}

// Text adds text content to the current element
func (b *XMLBuilder) Text(text string) *XMLBuilder {
	b.buf.WriteString(html.EscapeString(text))
	return b
}

// Element adds a complete element with text content
func (b *XMLBuilder) Element(name, text string, attrs ...string) *XMLBuilder {
	return b.StartElement(name, attrs...).Text(text).EndElement(name)
}

// writeIndent writes the current indentation
func (b *XMLBuilder) writeIndent() {
	for i := 0; i < b.indent; i++ {
		b.buf.WriteString("    ")
	}
}

// String returns the built XML as a string
func (b *XMLBuilder) String() string {
	return b.buf.String()
}
