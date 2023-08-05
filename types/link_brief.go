package types

// LinkBrief is the representation of any supported link in a brief format as defined in the topology file.
type LinkBrief struct {
	Endpoints        []string
	LinkCommonParams `yaml:",inline"`
}
