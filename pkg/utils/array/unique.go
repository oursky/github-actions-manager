package array

func Unique[T comparable](strSlice []T) []T {
	keys := make(map[T]bool)
	list := []T{}
	for _, entry := range strSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
