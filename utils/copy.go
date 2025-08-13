package utils

func CopyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}

	return out
}

func CopySlice(s []string) []string {
	if s == nil {
		return nil
	}

	out := make([]string, len(s))
	out = append(out[:0], s...)

	return out
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
