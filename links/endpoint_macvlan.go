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
func (e *EndpointMacVlan) Verify(ctx context.Context, _ *VerifyLinkParams) error {
	return CheckEndpointExists(ctx, e)
}

func (e *EndpointMacVlan) IsNodeless() bool {
	return false
}

func (e *EndpointMacVlan) MoveTo(ctx context.Context, dst Node) error {
	return moveEndpoint(ctx, e, dst)
}

func (e *EndpointMacVlan) Activate(ctx context.Context) error {
	return activateEndpoint(ctx, e)
}
