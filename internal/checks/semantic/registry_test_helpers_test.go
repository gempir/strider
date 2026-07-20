package semantic

func newRegistry(only []string) (*Registry, error) {
	return NewRegistry(RegistryOptions{
		Only: only,
	})
}
