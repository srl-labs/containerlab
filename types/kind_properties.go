package types

type KindProperties struct {
	// IsRootNamespaceBased flags if the Kind is run in the Hosts NetworkNamespace
	IsRootNamespaceBased bool
}

// NewKindProperties constructor for KindProperties
func NewKindProperties() *KindProperties {
	return &KindProperties{
		IsRootNamespaceBased: false,
	}
}
