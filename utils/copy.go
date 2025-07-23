package utils

func CopyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	copy := make(map[string]string, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

func CopySlice(s []string) []string {
	if s == nil {
		return nil
	}
	copy := make([]string, len(s))
	copy = append(copy[:0], s...)
	return copy
}

type CopyInterface[T any] interface{ Copy() T }

func CopyObjectSlice[T CopyInterface[T]](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	for i, v := range src {
		dst[i] = v.Copy()
	}
	return dst
}
