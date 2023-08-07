package links

type EndpointMacVlan struct {
	EndpointGeneric
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndpointMacVlan) Verify() error {
	return CheckEndpointExists(e)
}
