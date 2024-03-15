package links

import (
	"context"
	"errors"
)

type EndpointHost struct {
	EndpointGeneric
}

func NewEndpointHost(eg *EndpointGeneric) *EndpointHost {
	return &EndpointHost{
		EndpointGeneric: *eg,
	}
}

func (e *EndpointHost) Deploy(ctx context.Context) error {
	return e.GetLink().Deploy(ctx, e)
}

func (e *EndpointHost) Verify(ctx context.Context, _ *VerifyLinkParams) error {
	var errs []error
	err := CheckEndpointUniqueness(e)
	if err != nil {
		errs = append(errs, err)
	}
	err = CheckEndpointDoesNotExistYet(ctx, e)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (e *EndpointHost) IsNodeless() bool {
	return true
}
