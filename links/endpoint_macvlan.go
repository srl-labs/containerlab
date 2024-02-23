package links

import "context"

type EndpointMacVlan struct {
	EndpointGeneric
}

func NewEndpointMacVlan(eg *EndpointGeneric) *EndpointMacVlan {
	return &EndpointMacVlan{
		EndpointGeneric: *eg,
	}
}

func (e *EndpointMacVlan) Deploy(ctx context.Context) error {
	return e.GetLink().Deploy(ctx, e)
}

// Verify runs verification to check if the endpoint can be deployed.
func (e *EndpointMacVlan) Verify(_ *VerifyLinkParams) error {
	return CheckEndpointExists(e)
}

func (e *EndpointMacVlan) AutoDeployWithAEnd() bool {
	return false
}
