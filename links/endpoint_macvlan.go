package links

type EndpointMacVlan struct {
	EndpointGeneric
}

func NewEndpointMacVlan(eg *EndpointGeneric) *EndpointMacVlan {
	return &EndpointMacVlan{
		EndpointGeneric: *eg,
	}
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndpointMacVlan) Verify(_ *VerifyLinkParams) error {
	return CheckEndpointExists(e)
}
