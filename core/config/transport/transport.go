package transport

import (
	"fmt"
)

type TransportOption func(*Transport)

type Transport interface {
	// Connect to the target host
	Connect(host string, options ...TransportOption) error
	// Execute some config
	Write(data *string, info *string) error
	Close()
}

// Write config to a node.
func Write(tx Transport, host string, data, info []string, options ...TransportOption) error {
	// the Kind should configure the transport parameters before
	err := tx.Connect(host, options...)
	if err != nil {
		return fmt.Errorf("%s: %s", host, err)
	}

	defer tx.Close()

	for i1, d1 := range data {
		err := tx.Write(&d1, &info[i1])
		if err != nil {
			return fmt.Errorf("could not write config %s: %s", d1, err)
		}
	}

	return nil
}
