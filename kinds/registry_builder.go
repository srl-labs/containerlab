package kinds

type RegistryBuilder []func(registry *Registry) error

func (rb *RegistryBuilder) AddToRegistry(r *Registry) error {
	for _, f := range *rb {
		if err := f(r); err != nil {
			return err
		}
	}
	return nil
}
