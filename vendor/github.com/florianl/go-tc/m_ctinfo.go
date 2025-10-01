package tc

import (
	"fmt"

	"github.com/mdlayher/netlink"
)

const (
	tcaCtInfoUnspec = iota
	tcaCtInfoPad
	tcaCtInfoTm
	tcaCtInfoAct
	tcaCtInfoZone
	tcaCtInfoParmsDscpMask
	tcaCtInfoParmsDscpStateMask
	tcaCtInfoParmsCpMarkMask
	tcaCtInfoStatsDscpSet
	tcaCtInfoStatsDscpError
	tcaCtInfoStatsCpMarkSet
)

// CtInfo contains attributes of the ctinfo discipline
type CtInfo struct {
	Tm                 *Tcft
	Act                *CtInfoAct
	Zone               *uint16
	ParmsDscpMask      *uint32
	ParmsDscpStateMask *uint32
	ParmsCpMarkMask    *uint32
	StatsDscpSet       *uint64
	StatsDscpError     *uint64
	StatsCpMarkSet     *uint64
}

// CtInfoAct as tc_ctinfo from include/uapi/linux/tc_act/tc_ctinfo.h
type CtInfoAct struct {
	Index   uint32
	Capab   uint32
	Action  uint32
	RefCnt  uint32
	BindCnt uint32
}

// unmarshalCtInfo parses the ctinfo-encoded data and stores the result in the value pointed to by info.
func unmarshalCtInfo(data []byte, info *CtInfo) error {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	var multiError error
	for ad.Next() {
		switch ad.Type() {
		case tcaCtInfoTm:
			tcft := &Tcft{}
			err = unmarshalStruct(ad.Bytes(), tcft)
			multiError = concatError(multiError, err)
			info.Tm = tcft
		case tcaCtInfoAct:
			parms := &CtInfoAct{}
			err = unmarshalStruct(ad.Bytes(), parms)
			multiError = concatError(multiError, err)
			info.Act = parms
		case tcaCtInfoZone:
			info.Zone = uint16Ptr(ad.Uint16())
		case tcaCtInfoParmsDscpMask:
			info.ParmsDscpMask = uint32Ptr(ad.Uint32())
		case tcaCtInfoParmsDscpStateMask:
			info.ParmsDscpStateMask = uint32Ptr(ad.Uint32())
		case tcaCtInfoParmsCpMarkMask:
			info.ParmsCpMarkMask = uint32Ptr(ad.Uint32())
		case tcaCtInfoStatsDscpSet:
			info.StatsDscpSet = uint64Ptr(ad.Uint64())
		case tcaCtInfoStatsDscpError:
			info.StatsDscpError = uint64Ptr(ad.Uint64())
		case tcaCtInfoStatsCpMarkSet:
			info.StatsCpMarkSet = uint64Ptr(ad.Uint64())
		case tcaCtInfoPad:
			// padding does not contain data, we just skip it
		default:
			return fmt.Errorf("UnmarshalCtInfo()\t%d\n\t%v", ad.Type(), ad.Bytes())
		}
	}
	return concatError(multiError, ad.Err())
}

// marshalCtInfo returns the binary encoding of CtInfo
func marshalCtInfo(info *CtInfo) ([]byte, error) {
	options := []tcOption{}

	if info == nil {
		return []byte{}, fmt.Errorf("CtInfo: %w", ErrNoArg)
	}
	// TODO: improve logic and check combinations
	if info.Tm != nil {
		return []byte{}, ErrNoArgAlter
	}
	if info.Act != nil {
		data, err := marshalStruct(info.Act)
		if err != nil {
			return []byte{}, nil
		}
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaCtInfoAct, Data: data})
	}
	if info.Zone != nil {
		options = append(options, tcOption{Interpretation: vtUint16, Type: tcaCtInfoZone, Data: uint16Value(info.Zone)})

	}
	if info.ParmsDscpMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaCtInfoParmsDscpMask, Data: uint32Value(info.ParmsDscpMask)})
	}
	if info.ParmsDscpStateMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaCtInfoParmsDscpStateMask, Data: uint32Value(info.ParmsDscpStateMask)})
	}
	if info.ParmsCpMarkMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaCtInfoParmsCpMarkMask, Data: uint32Value(info.ParmsCpMarkMask)})
	}
	if info.StatsDscpSet != nil {
		options = append(options, tcOption{Interpretation: vtUint64, Type: tcaCtInfoStatsDscpSet, Data: uint64Value(info.StatsDscpSet)})
	}
	if info.StatsDscpError != nil {
		options = append(options, tcOption{Interpretation: vtUint64, Type: tcaCtInfoStatsDscpError, Data: uint64Value(info.StatsDscpError)})
	}
	if info.StatsCpMarkSet != nil {
		options = append(options, tcOption{Interpretation: vtUint64, Type: tcaCtInfoStatsCpMarkSet, Data: uint64Value(info.StatsCpMarkSet)})
	}
	return marshalAttributes(options)
}
