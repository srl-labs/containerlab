package tc

import (
	"fmt"
	"net"

	"github.com/mdlayher/netlink"
)

const (
	tcaSkbModUnspec = iota
	tcaSkbModTm
	tcaSkbModParms
	tcaSkbModDMac
	tcaSkbModSMac
	tcaSkbModEType
	tcaSkbModPad
)

// SkbMod contains attribute of thet SkbMod discipline
type SkbMod struct {
	Tm    *Tcft
	Parms *SkbModParms
	DMac  *net.HardwareAddr
	SMac  *net.HardwareAddr
	EType *uint16
}

// SkbModParms from include/uapi/linux/tc_act/tc_skbmod.h
type SkbModParms struct {
	Index   uint32
	Capab   uint32
	Action  uint32
	RefCnt  uint32
	BindCnt uint32
	Flags   uint64
}

func marshalSkbMod(info *SkbMod) ([]byte, error) {
	options := []tcOption{}
	if info == nil {
		return []byte{}, fmt.Errorf("skbmod: %w", ErrNoArg)
	}
	// TODO: improve logic and check combinations
	if info.Tm != nil {
		return []byte{}, ErrNoArgAlter
	}

	if info.Parms != nil {
		data, err := marshalStruct(info.Parms)
		if err != nil {
			return []byte{}, err
		}
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaSkbModParms, Data: data})
	}
	if info.DMac != nil {
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaSkbModDMac, Data: hardwareAddrToBytes(*info.DMac)})
	}
	if info.SMac != nil {
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaSkbModSMac, Data: hardwareAddrToBytes(*info.SMac)})
	}
	if info.EType != nil {
		options = append(options, tcOption{Interpretation: vtUint16, Type: tcaSkbModEType, Data: *info.EType})
	}

	return marshalAttributes(options)
}

func unmarshalSkbMod(data []byte, info *SkbMod) error {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	var multiError error
	for ad.Next() {
		switch ad.Type() {
		case tcaSkbModParms:
			parms := &SkbModParms{}
			err = unmarshalStruct(ad.Bytes(), parms)
			multiError = concatError(multiError, err)
			info.Parms = parms
		case tcaSkbModTm:
			tcft := &Tcft{}
			err = unmarshalStruct(ad.Bytes(), tcft)
			multiError = concatError(multiError, err)
			info.Tm = tcft
		case tcaSkbModDMac:
			mac := bytesToHardwareAddr(ad.Bytes())
			info.DMac = &mac
		case tcaSkbModSMac:
			mac := bytesToHardwareAddr(ad.Bytes())
			info.SMac = &mac
		case tcaSkbModEType:
			info.EType = uint16Ptr(ad.Uint16())
		case tcaSkbModPad:
			// padding does not contain data, we just skip it
		default:
			return fmt.Errorf("unmarshalSkbMod()\t%d\n\t%v", ad.Type(), ad.Bytes())
		}
	}
	return concatError(multiError, ad.Err())
}
