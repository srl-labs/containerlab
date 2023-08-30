package links

import "errors"

type EndpointHost struct {
	EndpointGeneric
}

func NewEndpointHost(eg *EndpointGeneric) *EndpointHost {
	return &EndpointHost{
		EndpointGeneric: *eg,
	}
}

func (e *EndpointHost) Verify(_ *VerifyLinkParams) error {
	var errs []error
	err := CheckEndpointUniqueness(e)
	if err != nil {
		errs = append(errs, err)
	}
	err = CheckEndpointDoesNotExistYet(e)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
