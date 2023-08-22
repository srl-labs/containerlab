package links

type EndpointMacVlan struct {
	EndpointGeneric
}

// Verify verifies the veth based deployment pre-conditions.
func (e *EndpointMacVlan) Verify(_ *VerifyLinkParams) error {
	return CheckEndpointExists(e)
}
