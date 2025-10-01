package netconf

import (
	"encoding/xml"
	"strconv"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

type commit struct {
	XMLName          xml.Name       `xml:"commit"`
	Confirmed        *targetElement `xml:"confirmed,omitempty"`
	ConfirmedTimeout string         `xml:"confirm-timeout,omitempty"`
	Persist          string         `xml:"persist,omitempty"`
	PersistID        string         `xml:"persist-id,omitempty"`
}

func (d *Driver) buildCommitElem(confirmed bool, timeout uint, persist, persistID string) *message {
	commitElem := &commit{
		XMLName:   xml.Name{},
		Persist:   persist,
		PersistID: persistID,
	}

	if confirmed {
		commitElem.Confirmed = &targetElement{}
	}

	if timeout > 0 {
		commitElem.ConfirmedTimeout = strconv.Itoa(int(timeout)) //nolint:gosec
	}

	netconfInput := d.buildPayload(commitElem)

	return netconfInput
}

// Commit executes a commit rpc against the NETCONF server.
func (d *Driver) Commit(opts ...util.Option) (*response.NetconfResponse, error) {
	d.Logger.Info("Commit RPC requested")

	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	m := d.buildCommitElem(
		op.CommitConfirmed,
		op.CommitConfirmTimeout,
		op.CommitConfirmedPersist,
		op.CommitConfirmedPersistID)

	return d.sendRPC(m, op)
}

type discard struct {
	XMLName xml.Name `xml:"discard-changes"`
}

func (d *Driver) buildDiscardElem() *message {
	discardElem := &discard{
		XMLName: xml.Name{},
	}

	netconfInput := d.buildPayload(discardElem)

	return netconfInput
}

// Discard executes a discard rpc against the NETCONF server.
func (d *Driver) Discard() (*response.NetconfResponse, error) {
	d.Logger.Info("Discard RPC requested")

	op, err := NewOperation()
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildDiscardElem(), op)
}
