package array

func Unique[T comparable](slice []T) []T {
	keys := make(map[T]struct{})
	var list []T
	for _, entry := range slice {
		if val, ok := keys[entry]; !ok {
			keys[entry] = val
			list = append(list, entry)
		}
	}
	return list
}
