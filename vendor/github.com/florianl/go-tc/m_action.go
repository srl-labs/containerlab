package tc

import (
	"errors"
	"fmt"

	"github.com/florianl/go-tc/internal/unix"
	"github.com/mdlayher/netlink"
)

const (
	tcaActUnspec = iota
	tcaActKind
	tcaActOptions
	tcaActIndex
	tcaActStats
	tcaActPad
	tcaActCookie
	tcaActFlags
	tcaActHwStats
	tcaActUsedHwStats
	tcaActInHwCount
)

// Various action binding types.
const (
	ActBind      = 1
	ActNoBind    = 0
	ActUnbind    = 1
	ActNoUnbind  = 0
	ActReplace   = 1
	ActNoReplace = 0
)

// Various action returns.
const (
	ActOk         = 0
	ActReclassify = 1
	ActShot       = 2
	ActPipe       = 3
	ActStolen     = 4
	ActQueued     = 5
	ActRepeat     = 6
	ActRedirect   = 7
	ActTrap       = 8
)

// Action represents action attributes of various filters and classes
type Action struct {
	Kind        string
	Index       uint32
	Stats       *GenStats
	Cookie      *[]byte
	Flags       *uint64 // 32-bit bitfield value; 32-bit bitfield selector
	HwStats     *uint64 // 32-bit bitfield value; 32-bit bitfield selector
	UsedHwStats *uint64 // 32-bit bitfield value; 32-bit bitfield selector
	InHwCount   *uint32

	Bpf       *ActBpf
	ConnMark  *Connmark
	CSum      *Csum
	Ct        *Ct
	CtInfo    *CtInfo
	Defact    *Defact
	Gact      *Gact
	Gate      *Gate
	Ife       *Ife
	Ipt       *Ipt
	Mirred    *Mirred
	Nat       *Nat
	Sample    *Sample
	VLan      *VLan
	Police    *Police
	TunnelKey *TunnelKey
	MPLS      *MPLS
	SkbEdit   *SkbEdit
	SkbMod    *SkbMod
}

func unmarshalActions(data []byte, actions *[]*Action) error {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	for ad.Next() {
		action := &Action{}
		if err := unmarshalAction(ad.Bytes(), action); err != nil {
			return err
		}
		*actions = append(*actions, action)
	}
	return ad.Err()
}

// unmarshalAction parses the Action-encoded data and stores the result in the value pointed to by info.
func unmarshalAction(data []byte, info *Action) error {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	var actOptions []byte
	for ad.Next() {
		switch ad.Type() {
		case tcaActKind:
			info.Kind = ad.String()
		case tcaActIndex:
			info.Index = ad.Uint32()
		case tcaActOptions:
			actOptions = ad.Bytes()
		case tcaActCookie:
			tmp := ad.Bytes()
			info.Cookie = &tmp
		case tcaActStats:
			stats := &GenStats{}
			if err := unmarshalGenStats(ad.Bytes(), stats); err != nil {
				return err
			}
			info.Stats = stats
		case tcaActFlags:
			flags := ad.Uint64()
			info.Flags = &flags
		case tcaActHwStats:
			hwStats := ad.Uint64()
			info.HwStats = &hwStats
		case tcaActUsedHwStats:
			usedHwStats := ad.Uint64()
			info.UsedHwStats = &usedHwStats
		case tcaActInHwCount:
			inHwCount := ad.Uint32()
			info.InHwCount = &inHwCount
		case tcaActPad:
			// padding does not contain data, we just skip it
		default:
			return fmt.Errorf("unmarshalAction()\t%d\n\t%v", ad.Type(), ad.Bytes())
		}
	}
	if len(actOptions) > 0 {
		if err := extractActOptions(actOptions, info, info.Kind); err != nil {
			return err
		}
	}

	return ad.Err()
}

func marshalActions(cmd int, info []*Action) ([]byte, error) {
	options := []tcOption{}

	for i, action := range info {
		data, err := marshalAction(cmd, action, tcaActOptions|nlaFNnested)
		if err != nil {
			return []byte{}, err
		}
		options = append(options, tcOption{Interpretation: vtBytes, Type: uint16(i + 1), Data: data})
	}

	return marshalAttributes(options)
}

// marshalAction returns the binary encoding of Action
func marshalAction(cmd int, info *Action, actOption uint16) ([]byte, error) {
	options := []tcOption{}

	if info == nil {
		return []byte{}, fmt.Errorf("Action: %w", ErrNoArg)
	}

	if len(info.Kind) == 0 {
		return []byte{}, fmt.Errorf("kind is missing")
	}
	var err error
	var data []byte

	// TODO: improve logic and check combinations
	switch info.Kind {
	case "bpf":
		data, err = marshalActBpf(info.Bpf)
	case "connmark":
		data, err = marshalConnmark(info.ConnMark)
	case "csum":
		data, err = marshalCsum(info.CSum)
	case "ct":
		data, err = marshalCt(info.Ct)
	case "ctinfo":
		data, err = marshalCtInfo(info.CtInfo)
	case "defact":
		data, err = marshalDefact(info.Defact)
	case "gact":
		data, err = marshalGact(info.Gact)
	case "gate":
		data, err = marshalGate(info.Gate)
	case "ife":
		data, err = marshalIfe(info.Ife)
	case "ipt":
		data, err = marshalIpt(info.Ipt)
	case "mirred":
		data, err = marshalMirred(info.Mirred)
	case "nat":
		data, err = marshalNat(info.Nat)
	case "sample":
		data, err = marshalSample(info.Sample)
	case "vlan":
		data, err = marshalVlan(info.VLan)
	case "police":
		data, err = marshalPolice(info.Police)
	case "tunnel_key":
		data, err = marshalTunnelKey(info.TunnelKey)
	case "mpls":
		data, err = marshalMPLS(info.MPLS)
	case "skbedit":
		data, err = marshalSkbEdit(info.SkbEdit)
	case "skbmod":
		data, err = marshalSkbMod(info.SkbMod)
	default:
		return []byte{}, fmt.Errorf("unknown kind '%s'", info.Kind)
	}

	if err != nil && !errors.Is(err, ErrNoArg) && cmd != unix.RTM_DELACTION {
		return []byte{}, err
	}

	options = append(options, tcOption{Interpretation: vtBytes, Type: actOption, Data: data})
	options = append(options, tcOption{Interpretation: vtString, Type: tcaActKind, Data: info.Kind})

	if info.Index != 0 {
		options = append(options, tcOption{Interpretation: vtUint32, Type: tcaActIndex, Data: info.Index})
	}
	if info.Stats != nil {
		data, err := marshalGenStats(info.Stats)
		if err != nil {
			return []byte{}, err
		}
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaActStats, Data: data})
	}
	if info.Cookie != nil {
		options = append(options, tcOption{Interpretation: vtBytes, Type: tcaActCookie, Data: bytesValue(info.Cookie)})
	}
	if info.Flags != nil {
		options = append(options, tcOption{Interpretation: vtUint64, Type: tcaActFlags, Data: uint64Value(info.Flags)})
	}
	return marshalAttributes(options)
}

func extractActOptions(data []byte, act *Action, kind string) error {
	var err error
	switch kind {
	case "bpf":
		info := &ActBpf{}
		err = unmarshalActBpf(data, info)
		act.Bpf = info
	case "connmark":
		info := &Connmark{}
		err = unmarshalConnmark(data, info)
		act.ConnMark = info
	case "csum":
		info := &Csum{}
		err = unmarshalCsum(data, info)
		act.CSum = info
	case "ct":
		info := &Ct{}
		err = unmarshalCt(data, info)
		act.Ct = info
	case "ctinfo":
		info := &CtInfo{}
		err = unmarshalCtInfo(data, info)
		act.CtInfo = info
	case "defact":
		info := &Defact{}
		err = unmarshalDefact(data, info)
		act.Defact = info
	case "gact":
		info := &Gact{}
		err = unmarshalGact(data, info)
		act.Gact = info
	case "gate":
		info := &Gate{}
		err = unmarshalGate(data, info)
		act.Gate = info
	case "ife":
		info := &Ife{}
		err = unmarshalIfe(data, info)
		act.Ife = info
	case "ipt":
		info := &Ipt{}
		err = unmarshalIpt(data, info)
		act.Ipt = info
	case "mirred":
		info := &Mirred{}
		err = unmarshalMirred(data, info)
		act.Mirred = info
	case "nat":
		info := &Nat{}
		err = unmarshalNat(data, info)
		act.Nat = info
	case "sample":
		info := &Sample{}
		err = unmarshalSample(data, info)
		act.Sample = info
	case "vlan":
		info := &VLan{}
		err = unmarshalVLan(data, info)
		act.VLan = info
	case "police":
		info := &Police{}
		err = unmarshalPolice(data, info)
		act.Police = info
	case "tunnel_key":
		info := &TunnelKey{}
		err = unmarshalTunnelKey(data, info)
		act.TunnelKey = info
	case "mpls":
		info := &MPLS{}
		err = unmarshalMPLS(data, info)
		act.MPLS = info
	case "skbedit":
		info := &SkbEdit{}
		err = unmarshalSkbEdit(data, info)
		act.SkbEdit = info
	case "skbmod":
		info := &SkbMod{}
		err = unmarshalSkbMod(data, info)
		act.SkbMod = info
	default:
		return fmt.Errorf("extractActOptions(): unsupported kind: %s", kind)

	}
	return err
}
