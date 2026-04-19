package links

import "context"

type EndpointVeth struct {
	EndpointGeneric
}

func NewEndpointVeth(eg *EndpointGeneric) *EndpointVeth {
	return &EndpointVeth{
		EndpointGeneric: *eg,
	}
}

// Verify verifies the veth based deployment pre-conditions.
func (e *EndpointVeth) Verify(_ context.Context, _ *VerifyLinkParams) error {
	return CheckEndpointUniqueness(e)
}

func (e *EndpointVeth) Deploy(ctx context.Context) error {
	return e.GetLink().Deploy(ctx, e)
}

func (e *EndpointVeth) IsNodeless() bool {
	return false
}
