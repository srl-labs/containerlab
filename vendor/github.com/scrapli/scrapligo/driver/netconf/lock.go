package netconf

import (
	"encoding/xml"

	"github.com/scrapli/scrapligo/response"
)

type lock struct {
	XMLName xml.Name `xml:"lock"`
	Target  *targetT `xml:""`
}

func (d *Driver) buildLockElem(target string) *message {
	lockElem := &lock{
		XMLName: xml.Name{},
		Target:  d.buildTargetElem(target),
	}

	netconfInput := d.buildPayload(lockElem)

	return netconfInput
}

// Lock executes the lock rpc for the target datastore against the NETCONF server.
func (d *Driver) Lock(target string) (*response.NetconfResponse, error) {
	d.Logger.Info("Lock RPC requested")

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildLockElem(target), op)
}

type unlock struct {
	XMLName xml.Name `xml:"unlock"`
	Target  *targetT `xml:""`
}

func (d *Driver) buildUnlockElem(target string) *message {
	unlockElem := &unlock{
		XMLName: xml.Name{},
		Target:  d.buildTargetElem(target),
	}

	netconfInput := d.buildPayload(unlockElem)

	return netconfInput
}

// Unlock executes unlock rpc for the target datastore against the NETCONF server.
func (d *Driver) Unlock(target string) (*response.NetconfResponse, error) {
	d.Logger.Info("Unlock RPC requested")

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildUnlockElem(target), op)
}
