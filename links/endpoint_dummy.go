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

func (e *EndpointDummy) MoveTo(ctx context.Context, dst Node, bringUp bool) error {
	return moveEndpoint(ctx, e, dst, bringUp)
}

func (*EndpointDummy) IsNodeless() bool {
	return false
}
