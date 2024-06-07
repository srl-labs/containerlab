package links

import "context"

type EndpointDummy struct {
	EndpointGeneric
}

func NewEndpointDummy(eg *EndpointGeneric) *EndpointDummy {
	return &EndpointDummy{
		EndpointGeneric: *eg,
	}
}

// Verify verifies the veth based deployment pre-conditions.
func (e *EndpointDummy) Verify(_ context.Context, _ *VerifyLinkParams) error {
	return CheckEndpointUniqueness(e)
}

func (e *EndpointDummy) Deploy(ctx context.Context) error {
	return e.GetLink().Deploy(ctx, e)
}

func (*EndpointDummy) IsNodeless() bool {
	return false
}
