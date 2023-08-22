package links

type EndpointVeth struct {
	EndpointGeneric
}

// Verify verifies the veth based deployment pre-conditions.
func (e *EndpointVeth) Verify(_ *VerifyLinkParams) error {
	return CheckEndpointUniqueness(e)
}
