package links

type EndpointMacVlan struct {
	EndpointGeneric
}

func NewEndpointMacVlan(eg *EndpointGeneric) *EndpointMacVlan {
	return &EndpointMacVlan{
		EndpointGeneric: *eg,
	}
}

// Verify runs verification to check if the endpoint can be deployed.
func (e *EndpointMacVlan) Verify(_ *VerifyLinkParams) error {
	return CheckEndpointExists(e)
}
