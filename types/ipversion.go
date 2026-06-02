package types

type IpVersion string

var (
	IpVersionAny IpVersion = "any"
	IpVersionV4  IpVersion = "ipv4"
	IpVersionV6  IpVersion = "ipv6"
)
