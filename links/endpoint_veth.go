package links

type EndpointVeth struct {
	EndpointGeneric
}

func NewEndpointVeth(eg *EndpointGeneric) *EndpointVeth {
	return &EndpointVeth{
		EndpointGeneric: *eg,
	}
}

// Verify verifies the veth based deployment pre-conditions.
func (e *EndpointVeth) Verify(_ *VerifyLinkParams) error {
	return CheckEndpointUniqueness(e)
}
