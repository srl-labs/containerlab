package tc

import (
	"fmt"
	"net"

	"github.com/mdlayher/netlink"
)

const (
	tcaFlowerUnspec = iota
	tcaFlowerClassID
	tcaFlowerIndev
	tcaFlowerAct
	tcaFlowerKeyEthDst
	tcaFlowerKeyEthDstMask
	tcaFlowerKeyEthSrc
	tcaFlowerKeyEthSrcMask
	tcaFlowerKeyEthType
	tcaFlowerKeyIPProto
	tcaFlowerKeyIPv4Src
	tcaFlowerKeyIPv4SrcMask
	tcaFlowerKeyIPv4Dst
	tcaFlowerKeyIPv4DstMask
	tcaFlowerKeyIPv6Src
	tcaFlowerKeyIPv6SrcMask
	tcaFlowerKeyIPv6Dst
	tcaFlowerKeyIPV6DstMask
	tcaFlowerKeyTCPSrc
	tcaFlowerKeyTCPDst
	tcaFlowerKeyUDPSrc
	tcaFlowerKeyUDPDst
	tcaFlowerFlags
	tcaFlowerKeyVlanID
	tcaFlowerKeyVlanPrio
	tcaFlowerKeyVlanEthType
	tcaFlowerKeyEncKeyID
	tcaFlowerKeyEncIPv4Src
	tcaFlowerKeyEncIPv4SrcMask
	tcaFlowerKeyEncIPv4Dst
	tcaFlowerKeyEncIPv4DstMask
	tcaFlowerKeyEncIPv6Src
	tcaFlowerKeyEncIPv6SrcMask
	tcaFlowerKeyEncIPv6Dst
	tcaFlowerKeyEncIPv6DstMask
	tcaFlowerKeyTCPSrcMask
	tcaFlowerKeyTCPDstMask
	tcaFlowerKeyUDPSrcMask
	tcaFlowerKeyUDPDstMask
	tcaFlowerKeySCTPSrcMask
	tcaFlowerKeySCTPDstMask
	tcaFlowerKeySCTPSrc
	tcaFlowerKeySCTPDst
	tcaFlowerKeyEncUDPSrcPort
	tcaFlowerKeyEncUDPSrcPortMask
	tcaFlowerKeyEncUDPDstPort
	tcaFlowerKeyEncUDPDstPortMask
	tcaFlowerKeyFlags
	tcaFlowerKeyFlagsMask
	tcaFlowerKeyIcmpv4Code
	tcaFlowerKeyIcmpv4CodeMask
	tcaFlowerKeyIcmpv4Type
	tcaFlowerKeyIcmpv4TypeMask
	tcaFlowerKeyIcmpv6Code
	tcaFlowerKeyIcmpv6CodeMask
	tcaFlowerKeyIcmpv6Type
	tcaFlowerKeyIcmpv6TypeMask
	tcaFlowerKeyArpSIP
	tcaFlowerKeyArpSIPMask
	tcaFlowerKeyArpTIP
	tcaFlowerKeyArpTIPMask
	tcaFlowerKeyArpOp
	tcaFlowerKeyArpOpMask
	tcaFlowerKeyArpSha
	tcaFlowerKeyArpShaMask
	tcaFlowerKeyArpTha
	tcaFlowerKeyArpThaMask
	tcaFlowerKeyMplsTTL
	tcaFlowerKeyMplsBos
	tcaFlowerKeyMplsTc
	tcaFlowerKeyMplsLabel
	tcaFlowerKeyTCPFlags
	tcaFlowerKeyTCPFlagsMask
	tcaFlowerKeyIPTOS
	tcaFlowerKeyIPTOSMask
	tcaFlowerKeyIPTTL
	tcaFlowerKeyIPTTLMask
	tcaFlowerKeyCVlanID
	tcaFlowerKeyCVlanPrio
	tcaFlowerKeyCVlanEthType
	tcaFlowerKeyEncIPTOS
	tcaFlowerKeyEncIPTOSMask
	tcaFlowerKeyEncIPTTL
	tcaFlowerKeyEncIPTTLMask
	tcaFlowerKeyEncOpts
	tcaFlowerKeyEncOptsMask
	tcaFlowerInHwCount
	tcaFlowerKeyPortSrcMin
	tcaFlowerKeyPortSrcMax
	tcaFlowerKeyPortDstMin
	tcaFlowerKeyPortDstMax
	tcaFlowerKeyCtState
	tcaFlowerKeyCtStateMask
	tcaFlowerKeyCtZone
	tcaFlowerKeyCtZoneMask
	tcaFlowerKeyCtMark
	tcaFlowerKeyCtMarkMask
	tcaFlowerKeyCtLabels
	tcaFlowerKeyCtLabelsMask
	tcaFlowerKeyMplsOpts
	tcaFlowerKeyHash
	tcaFlowerKeyHashMask
	tcaFlowerKeyNumOfVLANS
	tcaFlowerKeyPppoeSID
	tcaFlowerKeyPppProto
	tcaFlowerKeyL2TPV3SID
	tcaFlowerL2Miss
	tcaFlowerKeyCFM
	tcaFlowerKeySPI
	tcaFlowerKeySPIMask
	tcaFlowerKeyEncFlags
	tcaFlowerKeyEncFlagsMask
)

// Flower contains attrobutes of the flower discipline
type Flower struct {
	ClassID              *uint32
	Indev                *string
	Actions              *[]*Action
	KeyEthDst            *net.HardwareAddr
	KeyEthDstMask        *net.HardwareAddr
	KeyEthSrc            *net.HardwareAddr
	KeyEthSrcMask        *net.HardwareAddr
	KeyEthType           *uint16 /* be16 */
	KeyIPProto           *uint8
	KeyIPv4Src           *net.IP
	KeyIPv4SrcMask       *net.IP
	KeyIPv4Dst           *net.IP
	KeyIPv4DstMask       *net.IP
	KeyTCPSrc            *uint16 /* be16 */
	KeyTCPDst            *uint16 /* be16 */
	KeyUDPSrc            *uint16 /* be16 */
	KeyUDPDst            *uint16 /* be16 */
	Flags                *uint32
	KeyVlanID            *uint16 /* be16 */
	KeyVlanPrio          *uint8
	KeyVlanEthType       *uint16 /* be16 */
	KeyEncKeyID          *uint32 /* be32 */
	KeyEncIPv4Src        *net.IP
	KeyEncIPv4SrcMask    *net.IP
	KeyEncIPv4Dst        *net.IP
	KeyEncIPv4DstMask    *net.IP
	KeyTCPSrcMask        *uint16 /* be16 */
	KeyTCPDstMask        *uint16 /* be16 */
	KeyUDPSrcMask        *uint16 /* be16 */
	KeyUDPDstMask        *uint16 /* be16 */
	KeySctpSrc           *uint16 /* be16 */
	KeySctpDst           *uint16 /* be16 */
	KeyEncUDPSrcPort     *uint16 /* be16 */
	KeyEncUDPSrcPortMask *uint16 /* be16 */
	KeyEncUDPDstPort     *uint16 /* be16 */
	KeyEncUDPDstPortMask *uint16 /* be16 */
	KeyFlags             *uint32 /* be32 */
	KeyFlagsMask         *uint32 /* be32 */
	KeyIcmpv4Code        *uint8
	KeyIcmpv4CodeMask    *uint8
	KeyIcmpv4Type        *uint8
	KeyIcmpv4TypeMask    *uint8
	KeyIcmpv6Code        *uint8
	KeyIcmpv6CodeMask    *uint8
	KeyArpSIP            *uint32 /* be32 */
	KeyArpSIPMask        *uint32 /* be32 */
	KeyArpTIP            *uint32 /* be32 */
	KeyArpTIPMask        *uint32 /* be32 */
	KeyArpOp             *uint8
	KeyArpOpMask         *uint8
	KeyMplsTTL           *uint8
	KeyMplsBos           *uint8
	KeyMplsTc            *uint8
	KeyMplsLabel         *uint32
	KeyTCPFlags          *uint16 /* be16 */
	KeyTCPFlagsMask      *uint16 /* be16 */
	KeyIPTOS             *uint8
	KeyIPTOSMask         *uint8
	KeyIPTTL             *uint8
	KeyIPTTLMask         *uint8
	KeyCVlanID           *uint16 /* be16 */
	KeyCVlanPrio         *uint8
	KeyCVlanEthType      *uint16 /* be16 */
	KeyEncIPTOS          *uint8
	KeyEncIPTOSMask      *uint8
	KeyEncIPTTL          *uint8
	KeyEncIPTTLMask      *uint8
	InHwCount            *uint32
	KeyPortSrcMin        *uint16 /* be16 */
	KeyPortSrcMax        *uint16 /* be16 */
	KeyPortDstMin        *uint16 /* be16 */
	KeyPortDstMax        *uint16 /* be16 */

	KeyCtState     *uint16 /* u16 */
	KeyCtStateMask *uint16 /* u16 */
	KeyCtZone      *uint16 /* u16 */
	KeyCtZoneMask  *uint16 /* u16 */
	KeyCtMark      *uint32 /* u32 */
	KeyCtMarkMask  *uint32 /* u32 */
	//KeyCtLabels,	/* u128 */
	//KeyCtLabelsMask,	/* u128 */
	//KeyMplsOpts,

	KeyHash     *uint32 /* u32 */
	KeyHashMask *uint32 /* u32 */

	KeyNumOfVLANS *uint8 /* u8 */

	KeyPppoeSID *uint16 /* be16 */
	KeyPppProto *uint16 /* be16 */

	KeyL2TPV3SID *uint32 /* be32 */

	L2Miss *uint8 /* u8 */

	//KeyCfm,		/* nested */

	KeySpi     *uint32 /* be32 */
	KeySpiMask *uint32 /* be32 */

	KeyEncFlags     *uint32 /* be32 */
	KeyEncFlagsMask *uint32 /* be32 */
}

// unmarshalFlower parses the Flower-encoded data and stores the result in the value pointed to by info.
func unmarshalFlower(data []byte, info *Flower) error {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	var multiError error
	for ad.Next() {
		switch ad.Type() {
		case tcaFlowerClassID:
			tmp := ad.Uint32()
			info.ClassID = &tmp
		case tcaFlowerIndev:
			tmp := ad.String()
			info.Indev = &tmp
		case tcaFlowerAct:
			actions := &[]*Action{}
			err := unmarshalActions(ad.Bytes(), actions)
			multiError = concatError(multiError, err)
			info.Actions = actions
		case tcaFlowerKeyEthDst:
			tmp := bytesToHardwareAddr(ad.Bytes())
			info.KeyEthDst = &tmp
		case tcaFlowerKeyEthDstMask:
			tmp := bytesToHardwareAddr(ad.Bytes())
			info.KeyEthDstMask = &tmp
		case tcaFlowerKeyEthSrc:
			tmp := bytesToHardwareAddr(ad.Bytes())
			info.KeyEthSrc = &tmp
		case tcaFlowerKeyEthSrcMask:
			tmp := bytesToHardwareAddr(ad.Bytes())
			info.KeyEthSrcMask = &tmp
		case tcaFlowerKeyEthType:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyEthType = &tmp
		case tcaFlowerKeyIPProto:
			tmp := ad.Uint8()
			info.KeyIPProto = &tmp
		case tcaFlowerKeyIPv4Src:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyIPv4Src = &tmp
		case tcaFlowerKeyIPv4SrcMask:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyIPv4SrcMask = &tmp
		case tcaFlowerKeyIPv4Dst:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyIPv4Dst = &tmp
		case tcaFlowerKeyIPv4DstMask:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyIPv4DstMask = &tmp
		case tcaFlowerKeyTCPSrc:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyTCPSrc = &tmp
		case tcaFlowerKeyTCPDst:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyTCPDst = &tmp
		case tcaFlowerKeyUDPSrc:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyUDPSrc = &tmp
		case tcaFlowerKeyUDPDst:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyUDPDst = &tmp
		case tcaFlowerFlags:
			tmp := ad.Uint32()
			info.Flags = &tmp
		case tcaFlowerKeyVlanID:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyVlanID = &tmp
		case tcaFlowerKeyVlanPrio:
			tmp := ad.Uint8()
			info.KeyVlanPrio = &tmp
		case tcaFlowerKeyVlanEthType:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyVlanEthType = &tmp
		case tcaFlowerKeyEncKeyID:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyEncKeyID = &tmp
		case tcaFlowerKeyEncIPv4Src:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyEncIPv4Src = &tmp
		case tcaFlowerKeyEncIPv4SrcMask:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyEncIPv4SrcMask = &tmp
		case tcaFlowerKeyEncIPv4Dst:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyEncIPv4Dst = &tmp
		case tcaFlowerKeyEncIPv4DstMask:
			tmp := uint32ToIP(ad.Uint32())
			info.KeyEncIPv4DstMask = &tmp
		case tcaFlowerKeyTCPSrcMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyTCPSrcMask = &tmp
		case tcaFlowerKeyTCPDstMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyTCPDstMask = &tmp
		case tcaFlowerKeyUDPSrcMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyUDPSrcMask = &tmp
		case tcaFlowerKeyUDPDstMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyUDPDstMask = &tmp
		case tcaFlowerKeySCTPSrc:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeySctpSrc = &tmp
		case tcaFlowerKeySCTPDst:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeySctpDst = &tmp
		case tcaFlowerKeyEncUDPSrcPort:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyEncUDPSrcPort = &tmp
		case tcaFlowerKeyEncUDPSrcPortMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyEncUDPSrcPortMask = &tmp
		case tcaFlowerKeyEncUDPDstPort:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyEncUDPDstPort = &tmp
		case tcaFlowerKeyEncUDPDstPortMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyEncUDPDstPortMask = &tmp
		case tcaFlowerKeyFlags:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyFlags = &tmp
		case tcaFlowerKeyFlagsMask:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyFlagsMask = &tmp
		case tcaFlowerKeyIcmpv4Code:
			tmp := ad.Uint8()
			info.KeyIcmpv4Code = &tmp
		case tcaFlowerKeyIcmpv4CodeMask:
			tmp := ad.Uint8()
			info.KeyIcmpv4CodeMask = &tmp
		case tcaFlowerKeyIcmpv4Type:
			tmp := ad.Uint8()
			info.KeyIcmpv4Type = &tmp
		case tcaFlowerKeyIcmpv4TypeMask:
			tmp := ad.Uint8()
			info.KeyIcmpv4TypeMask = &tmp
		case tcaFlowerKeyIcmpv6Code:
			tmp := ad.Uint8()
			info.KeyIcmpv6Code = &tmp
		case tcaFlowerKeyIcmpv6CodeMask:
			tmp := ad.Uint8()
			info.KeyIcmpv6CodeMask = &tmp
		case tcaFlowerKeyArpSIP:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyArpSIP = &tmp
		case tcaFlowerKeyArpSIPMask:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyArpSIPMask = &tmp
		case tcaFlowerKeyArpTIP:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyArpTIP = &tmp
		case tcaFlowerKeyArpTIPMask:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyArpTIPMask = &tmp
		case tcaFlowerKeyArpOp:
			tmp := ad.Uint8()
			info.KeyArpOp = &tmp
		case tcaFlowerKeyArpOpMask:
			tmp := ad.Uint8()
			info.KeyArpOpMask = &tmp
		case tcaFlowerKeyMplsTTL:
			tmp := ad.Uint8()
			info.KeyMplsTTL = &tmp
		case tcaFlowerKeyMplsBos:
			tmp := ad.Uint8()
			info.KeyMplsBos = &tmp
		case tcaFlowerKeyMplsTc:
			tmp := ad.Uint8()
			info.KeyMplsTc = &tmp
		case tcaFlowerKeyMplsLabel:
			tmp := ad.Uint32()
			info.KeyMplsLabel = &tmp
		case tcaFlowerKeyTCPFlags:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyTCPFlags = &tmp
		case tcaFlowerKeyTCPFlagsMask:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyTCPFlagsMask = &tmp
		case tcaFlowerKeyIPTOS:
			tmp := ad.Uint8()
			info.KeyIPTOS = &tmp
		case tcaFlowerKeyIPTOSMask:
			tmp := ad.Uint8()
			info.KeyIPTOSMask = &tmp
		case tcaFlowerKeyIPTTL:
			tmp := ad.Uint8()
			info.KeyIPTTL = &tmp
		case tcaFlowerKeyIPTTLMask:
			tmp := ad.Uint8()
			info.KeyIPTTLMask = &tmp
		case tcaFlowerKeyCVlanID:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyCVlanID = &tmp
		case tcaFlowerKeyCVlanPrio:
			tmp := ad.Uint8()
			info.KeyCVlanPrio = &tmp
		case tcaFlowerKeyCVlanEthType:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyCVlanEthType = &tmp
		case tcaFlowerKeyEncIPTOS:
			tmp := ad.Uint8()
			info.KeyEncIPTOS = &tmp
		case tcaFlowerKeyEncIPTOSMask:
			tmp := ad.Uint8()
			info.KeyEncIPTOSMask = &tmp
		case tcaFlowerKeyEncIPTTL:
			tmp := ad.Uint8()
			info.KeyEncIPTTL = &tmp
		case tcaFlowerKeyEncIPTTLMask:
			tmp := ad.Uint8()
			info.KeyEncIPTTLMask = &tmp
		case tcaFlowerInHwCount:
			tmp := ad.Uint32()
			info.InHwCount = &tmp
		case tcaFlowerKeyPortSrcMin:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyPortSrcMin = &tmp
		case tcaFlowerKeyPortSrcMax:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyPortSrcMax = &tmp
		case tcaFlowerKeyPortDstMin:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyPortDstMin = &tmp
		case tcaFlowerKeyPortDstMax:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyPortDstMax = &tmp
		case tcaFlowerKeyCtState:
			tmp := ad.Uint16()
			info.KeyCtState = &tmp
		case tcaFlowerKeyCtStateMask:
			tmp := ad.Uint16()
			info.KeyCtStateMask = &tmp
		case tcaFlowerKeyCtZone:
			tmp := ad.Uint16()
			info.KeyCtZone = &tmp
		case tcaFlowerKeyCtZoneMask:
			tmp := ad.Uint16()
			info.KeyCtZoneMask = &tmp
		case tcaFlowerKeyCtMark:
			tmp := ad.Uint32()
			info.KeyCtMark = &tmp
		case tcaFlowerKeyCtMarkMask:
			tmp := ad.Uint32()
			info.KeyCtMarkMask = &tmp
		case tcaFlowerKeyHash:
			tmp := ad.Uint32()
			info.KeyHash = &tmp
		case tcaFlowerKeyHashMask:
			tmp := ad.Uint32()
			info.KeyHashMask = &tmp
		case tcaFlowerKeyNumOfVLANS:
			tmp := ad.Uint8()
			info.KeyNumOfVLANS = &tmp
		case tcaFlowerKeyPppoeSID:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyPppoeSID = &tmp
		case tcaFlowerKeyPppProto:
			tmp := endianSwapUint16(ad.Uint16())
			info.KeyPppProto = &tmp
		case tcaFlowerKeyL2TPV3SID:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyL2TPV3SID = &tmp
		case tcaFlowerL2Miss:
			tmp := ad.Uint8()
			info.L2Miss = &tmp
		case tcaFlowerKeySPI:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeySpi = &tmp
		case tcaFlowerKeySPIMask:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeySpiMask = &tmp
		case tcaFlowerKeyEncFlags:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyEncFlags = &tmp
		case tcaFlowerKeyEncFlagsMask:
			tmp := endianSwapUint32(ad.Uint32())
			info.KeyEncFlagsMask = &tmp
		default:
			return fmt.Errorf("unmarshalFlower()\t%d\n\t%v", ad.Type(), ad.Bytes())
		}
	}
	return concatError(multiError, ad.Err())
}

// marshalFlower returns the binary encoding of Flow
func marshalFlower(info *Flower) ([]byte, error) {
	options := []tcOption{}

	if info == nil {
		return []byte{}, fmt.Errorf("Flower: %w", ErrNoArg)
	}
	var multiError error
	// TODO: improve logic and check combinations
	if info.ClassID != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerClassID, Data: *info.ClassID})
	}
	if info.Indev != nil {
		options = append(options, tcOption{Interpretation: vtString, Type: tcaFlowerIndev, Data: *info.Indev})
	}
	if info.Actions != nil {
		data, err := marshalActions(0, *info.Actions)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaFlowerAct, Data: data})
	}
	if info.KeyEthDst != nil {
		tmp := hardwareAddrToBytes(*info.KeyEthDst)
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaFlowerKeyEthDst, Data: tmp})
	}
	if info.KeyEthDstMask != nil {
		tmp := hardwareAddrToBytes(*info.KeyEthDstMask)
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaFlowerKeyEthDstMask, Data: tmp})
	}
	if info.KeyEthSrc != nil {
		tmp := hardwareAddrToBytes(*info.KeyEthSrc)
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaFlowerKeyEthSrc, Data: tmp})
	}
	if info.KeyEthSrcMask != nil {
		tmp := hardwareAddrToBytes(*info.KeyEthSrcMask)
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaFlowerKeyEthSrcMask, Data: tmp})
	}
	if info.KeyEthType != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyEthType, Data: *info.KeyEthType})
	}
	if info.KeyIPProto != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIPProto, Data: *info.KeyIPProto})
	}
	if info.KeyIPv4Src != nil {
		tmp, err := ipToUint32(*info.KeyIPv4Src)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyIPv4Src, Data: tmp})
	}
	if info.KeyIPv4SrcMask != nil {
		tmp, err := ipToUint32(*info.KeyIPv4SrcMask)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyIPv4SrcMask, Data: tmp})
	}
	if info.KeyIPv4Dst != nil {
		tmp, err := ipToUint32(*info.KeyIPv4Dst)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyIPv4Dst, Data: tmp})
	}
	if info.KeyIPv4DstMask != nil {
		tmp, err := ipToUint32(*info.KeyIPv4DstMask)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyIPv4DstMask, Data: tmp})
	}
	if info.KeyTCPSrc != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyTCPSrc, Data: *info.KeyTCPSrc})
	}
	if info.KeyTCPDst != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyTCPDst, Data: *info.KeyTCPDst})
	}
	if info.KeyUDPSrc != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyUDPSrc, Data: *info.KeyUDPSrc})
	}
	if info.KeyUDPDst != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyUDPDst, Data: *info.KeyUDPDst})
	}
	if info.Flags != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerFlags, Data: *info.Flags})
	}
	if info.KeyVlanID != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyVlanID, Data: *info.KeyVlanID})
	}
	if info.KeyVlanPrio != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyVlanPrio, Data: *info.KeyVlanPrio})
	}
	if info.KeyVlanEthType != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyVlanEthType, Data: *info.KeyVlanEthType})
	}
	if info.KeyEncKeyID != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyEncKeyID, Data: *info.KeyEncKeyID})
	}
	if info.KeyEncIPv4Src != nil {
		tmp, err := ipToUint32(*info.KeyEncIPv4Src)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyEncIPv4Src, Data: tmp})
	}
	if info.KeyEncIPv4SrcMask != nil {
		tmp, err := ipToUint32(*info.KeyEncIPv4SrcMask)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyEncIPv4SrcMask, Data: tmp})
	}
	if info.KeyEncIPv4Dst != nil {
		tmp, err := ipToUint32(*info.KeyEncIPv4Dst)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyEncIPv4Dst, Data: tmp})
	}
	if info.KeyEncIPv4DstMask != nil {
		tmp, err := ipToUint32(*info.KeyEncIPv4DstMask)
		multiError = concatError(multiError, err)
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyEncIPv4DstMask, Data: tmp})
	}
	if info.KeyTCPSrcMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyTCPSrcMask, Data: *info.KeyTCPSrcMask})
	}
	if info.KeyTCPDstMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyTCPDstMask, Data: *info.KeyTCPDstMask})
	}
	if info.KeyUDPSrcMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyUDPSrcMask, Data: *info.KeyUDPSrcMask})
	}
	if info.KeyUDPDstMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyUDPDstMask, Data: *info.KeyUDPDstMask})
	}
	if info.KeySctpSrc != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeySCTPSrc, Data: *info.KeySctpSrc})
	}
	if info.KeySctpDst != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeySCTPDst, Data: *info.KeySctpDst})
	}
	if info.KeyEncUDPSrcPort != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyEncUDPSrcPort, Data: *info.KeyEncUDPSrcPort})
	}
	if info.KeyEncUDPSrcPortMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyEncUDPSrcPortMask, Data: *info.KeyEncUDPSrcPortMask})
	}
	if info.KeyEncUDPDstPort != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyEncUDPDstPort, Data: *info.KeyEncUDPDstPort})
	}
	if info.KeyEncUDPDstPortMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyEncUDPDstPortMask, Data: *info.KeyEncUDPDstPortMask})
	}
	if info.KeyFlags != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyFlags, Data: *info.KeyFlags})
	}
	if info.KeyFlagsMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyFlagsMask, Data: *info.KeyFlagsMask})
	}
	if info.KeyIcmpv4Code != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIcmpv4Code, Data: *info.KeyIcmpv4Code})
	}
	if info.KeyIcmpv4CodeMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIcmpv4CodeMask, Data: *info.KeyIcmpv4CodeMask})
	}
	if info.KeyIcmpv4Type != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIcmpv4Type, Data: *info.KeyIcmpv4Type})
	}
	if info.KeyIcmpv4TypeMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIcmpv4TypeMask, Data: *info.KeyIcmpv4TypeMask})
	}
	if info.KeyIcmpv6Code != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIcmpv6Code, Data: *info.KeyIcmpv6Code})
	}
	if info.KeyIcmpv6CodeMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIcmpv6CodeMask, Data: *info.KeyIcmpv6CodeMask})
	}
	if info.KeyArpSIP != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyArpSIP, Data: *info.KeyArpSIP})
	}
	if info.KeyArpSIPMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyArpSIPMask, Data: *info.KeyArpSIPMask})
	}
	if info.KeyArpTIP != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyArpTIP, Data: *info.KeyArpTIP})
	}
	if info.KeyArpTIPMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyArpTIPMask, Data: *info.KeyArpTIPMask})
	}
	if info.KeyArpOp != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyArpOp, Data: *info.KeyArpOp})
	}
	if info.KeyArpOpMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyArpOpMask, Data: *info.KeyArpOpMask})
	}
	if info.KeyMplsTTL != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyMplsTTL, Data: *info.KeyMplsTTL})
	}
	if info.KeyMplsBos != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyMplsBos, Data: *info.KeyMplsBos})
	}
	if info.KeyMplsTc != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyMplsTc, Data: *info.KeyMplsTc})
	}
	if info.KeyMplsLabel != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyMplsLabel, Data: *info.KeyMplsLabel})
	}
	if info.KeyTCPFlags != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyTCPFlags, Data: *info.KeyTCPFlags})
	}
	if info.KeyTCPFlagsMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyTCPFlagsMask, Data: *info.KeyTCPFlagsMask})
	}
	if info.KeyIPTOS != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIPTOS, Data: *info.KeyIPTOS})
	}
	if info.KeyIPTOSMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIPTOSMask, Data: *info.KeyIPTOSMask})
	}
	if info.KeyIPTTL != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIPTTL, Data: *info.KeyIPTTL})
	}
	if info.KeyIPTTLMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyIPTTLMask, Data: *info.KeyIPTTLMask})
	}
	if info.KeyCVlanID != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyCVlanID, Data: *info.KeyCVlanID})
	}
	if info.KeyCVlanPrio != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyCVlanPrio, Data: *info.KeyCVlanPrio})
	}
	if info.KeyCVlanEthType != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyCVlanEthType, Data: *info.KeyCVlanEthType})
	}
	if info.KeyEncIPTOS != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyEncIPTOS, Data: *info.KeyEncIPTOS})
	}
	if info.KeyEncIPTOSMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyEncIPTOSMask, Data: *info.KeyEncIPTOSMask})
	}
	if info.KeyEncIPTTL != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyEncIPTTL, Data: *info.KeyEncIPTTL})
	}
	if info.KeyEncIPTTLMask != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyEncIPTTLMask, Data: *info.KeyEncIPTTLMask})
	}
	if info.InHwCount != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerInHwCount, Data: *info.InHwCount})
	}
	if info.KeyPortSrcMin != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyPortSrcMin, Data: *info.KeyPortSrcMin})
	}
	if info.KeyPortSrcMax != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyPortSrcMax, Data: *info.KeyPortSrcMax})
	}
	if info.KeyPortDstMin != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyPortDstMin, Data: *info.KeyPortDstMin})
	}
	if info.KeyPortDstMax != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyPortDstMax, Data: *info.KeyPortDstMax})
	}
	if info.KeyCtState != nil {
		options = append(options, tcOption{Interpretation: vtUint16, Type: tcaFlowerKeyCtState, Data: *info.KeyCtState})
	}
	if info.KeyCtStateMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16, Type: tcaFlowerKeyCtStateMask, Data: *info.KeyCtStateMask})
	}
	if info.KeyCtZone != nil {
		options = append(options, tcOption{Interpretation: vtUint16, Type: tcaFlowerKeyCtZone, Data: *info.KeyCtZone})
	}
	if info.KeyCtZoneMask != nil {
		options = append(options, tcOption{Interpretation: vtUint16, Type: tcaFlowerKeyCtZoneMask, Data: *info.KeyCtZoneMask})
	}
	if info.KeyCtMark != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyCtMark, Data: *info.KeyCtMark})
	}
	if info.KeyCtMarkMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyCtMarkMask, Data: *info.KeyCtMarkMask})
	}
	if info.KeyHash != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyHash, Data: *info.KeyHash})
	}
	if info.KeyHashMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaFlowerKeyHashMask, Data: *info.KeyHashMask})
	}
	if info.KeyNumOfVLANS != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerKeyNumOfVLANS, Data: *info.KeyNumOfVLANS})
	}
	if info.KeyPppoeSID != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyPppoeSID, Data: *info.KeyPppoeSID})
	}
	if info.KeyPppProto != nil {
		options = append(options, tcOption{Interpretation: vtUint16Be, Type: tcaFlowerKeyPppProto, Data: *info.KeyPppProto})
	}
	if info.KeyL2TPV3SID != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyL2TPV3SID, Data: *info.KeyL2TPV3SID})
	}
	if info.L2Miss != nil {
		options = append(options, tcOption{Interpretation: vtUint8, Type: tcaFlowerL2Miss, Data: *info.L2Miss})
	}
	if info.KeySpi != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeySPI, Data: *info.KeySpi})
	}
	if info.KeySpiMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeySPIMask, Data: *info.KeySpiMask})
	}
	if info.KeyEncFlags != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyEncFlags, Data: *info.KeyEncFlags})
	}
	if info.KeyEncFlagsMask != nil {
		options = append(options, tcOption{Interpretation: vtUint32Be, Type: tcaFlowerKeyEncFlagsMask, Data: *info.KeyEncFlagsMask})
	}
	if multiError != nil {
		return []byte{}, multiError
	}
	return marshalAttributes(options)
}
