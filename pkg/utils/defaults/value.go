package defaults

func Value[T any](value *T, defaultValue T) T {
	if value == nil {
		return defaultValue
	}
	return *value
}
