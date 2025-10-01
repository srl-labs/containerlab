package platform

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/generic"
	"github.com/scrapli/scrapligo/driver/network"

	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/util"
)

const (
	// OpChannelWrite is a constant that represents the channel write operation string in a
	// platform definition file.
	OpChannelWrite = "channel.write"
	// OpChannelReturn is a constant that represents the channel return operation string in a
	// platform definition file.
	OpChannelReturn = "channel.return"
	// OpAcquirePriv is a constant that represents the network driver acquire priv operation string
	// in a platform definition file.
	OpAcquirePriv = "acquire-priv"
	// OpDriverSendCommand is a constant that represents the generic or network driver send command
	// operation in a platform definition file.
	OpDriverSendCommand = "driver.send-command"
)

type onXDefinitions []map[string]interface{}

func channelWrite(op map[string]interface{}, c *channel.Channel) error {
	i, ok := op["input"].(string)
	if !ok {
		return fmt.Errorf("%w: bad value", util.ErrBadOption)
	}

	r, ok := op["redacted"].(bool)
	if !ok {
		r = false
	}

	return c.Write([]byte(i), r)
}

func (o *onXDefinitions) asGenericOnX() func(d *generic.Driver) error {
	return func(d *generic.Driver) error {
		for _, op := range *o {
			var err error

			opType, ok := op["operation"].(string)
			if !ok {
				panic("operation is invalid type, must be string!")
			}

			switch opType {
			case OpChannelWrite:
				err = channelWrite(op, d.Channel)
			case OpChannelReturn:
				err = d.Channel.WriteReturn()
			}

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func (o *onXDefinitions) asNetworkOnX() func(d *network.Driver) error {
	return func(d *network.Driver) error {
		for _, op := range *o {
			var err error

			opType, ok := op["operation"].(string)
			if !ok {
				panic("operation is invalid type, must be string!")
			}

			switch opType {
			case OpChannelWrite:
				err = channelWrite(op, d.Channel)
			case OpChannelReturn:
				err = d.Channel.WriteReturn()
			case OpAcquirePriv:
				target := d.DefaultDesiredPriv

				t, ok := op["target"].(string)
				if ok {
					target = t
				}

				err = d.AcquirePriv(target)
			case OpDriverSendCommand:
				c, ok := op["command"].(string)
				if !ok {
					return fmt.Errorf("%w: bad value", util.ErrBadOption)
				}

				_, err = d.SendCommand(c)
			}

			if err != nil {
				return err
			}
		}

		return nil
	}
}
