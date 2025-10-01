package tc

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/mdlayher/netlink"
)

type valueType int

const (
	vtUint8 valueType = iota
	vtUint16
	vtUint32
	vtUint64
	vtString
	vtBytes
	vtFlag
	vtInt8
	vtInt16
	vtInt32
	vtInt64
	vtUint16Be
	vtUint32Be
	vtInt16Be
)

type tcOption struct {
	Interpretation valueType
	Type           uint16
	Data           interface{}
}

// NLA_F_NESTED from include/uapi/linux/netlink.h
const nlaFNnested = (1 << 15)

func marshalAttributes(options []tcOption) ([]byte, error) {
	if len(options) == 0 {
		return []byte{}, nil
	}
	var multiError error
	ad := netlink.NewAttributeEncoder()

	for _, option := range options {
		switch option.Interpretation {
		case vtUint8:
			ad.Uint8(option.Type, (option.Data).(uint8))
		case vtUint16:
			ad.Uint16(option.Type, (option.Data).(uint16))
		case vtUint32:
			ad.Uint32(option.Type, (option.Data).(uint32))
		case vtUint64:
			ad.Uint64(option.Type, (option.Data).(uint64))
		case vtString:
			ad.String(option.Type, (option.Data).(string))
		case vtBytes:
			ad.Bytes(option.Type, (option.Data).([]byte))
		case vtFlag:
			ad.Flag(option.Type, true)
		case vtInt8:
			ad.Int8(option.Type, (option.Data).(int8))
		case vtInt16:
			ad.Int16(option.Type, (option.Data).(int16))
		case vtInt32:
			ad.Int32(option.Type, (option.Data).(int32))
		case vtInt64:
			ad.Int64(option.Type, (option.Data).(int64))
		case vtUint16Be:
			ad.Uint16(option.Type, endianSwapUint16((option.Data).(uint16)))
		case vtUint32Be:
			ad.Uint32(option.Type, endianSwapUint32((option.Data).(uint32)))
		case vtInt16Be:
			ad.Uint16(option.Type, endianSwapUint16(uint16((option.Data).(int16))))
		default:
			multiError = fmt.Errorf("unknown interpretation (%d)", option.Interpretation)
		}
	}
	if multiError != nil {
		return []byte{}, multiError
	}
	return ad.Encode()
}

func unmarshalNetlinkAttribute(data []byte, val interface{}) error {
	buf := bytes.NewReader(data)
	err := binary.Read(buf, nativeEndian, val)
	return err
}
