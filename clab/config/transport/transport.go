package transport

import (
	"fmt"
)

// Debug count
var DebugCount int

type Transport interface {
	// Connect to the target host
	Connect(host string, options ...func(*Transport)) error
	// Execute some config
	Write(data *string, info *string) error
	Close()
}

func Write(tx Transport, host string, data, info []string, options ...func(*Transport)) error {
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
