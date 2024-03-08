package types

func MapCopy[M1 ~map[K]V, M2 ~map[K]V, K comparable, V any](dst M1, src M2) {
	for k, v := range src {
		dst[k] = v
	}
}

func CopyBytes(bz []byte) (dst []byte) {
	dst = make([]byte, len(bz))
	copy(dst, bz)
	return
}
