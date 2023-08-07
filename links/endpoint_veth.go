package links

type EndpointVeth struct {
	EndpointGeneric
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndpointVeth) Verify() error {
	return CheckEndpointUniqueness(e)
}
