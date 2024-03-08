package types

import (
	"math/big"
)

func MapCopy[M1 ~map[K]V, M2 ~map[K]V, K comparable, V any](dst M1, src M2) {
	for k, v := range src {
		dst[k] = v
	}
}

func BigAdd(a, b *big.Int) *big.Int          { return new(big.Int).Add(a, b) }
func BigSub(a, b *big.Int) *big.Int          { return new(big.Int).Sub(a, b) }
func BigMul(a, b *big.Int) *big.Int          { return new(big.Int).Mul(a, b) }
func BigDiv(a, b *big.Int) *big.Int          { return new(big.Int).Div(a, b) }
func BigIsZero(a *big.Int) bool              { return len(a.Bits()) == 0 }
func BigEqual(a, b *big.Int) bool            { return a.Cmp(b) == 0 }
func BigLTE(a, b *big.Int) bool              { return a.Cmp(b) <= 0 }
func BigGTE(a *big.Int, b *big.Int) bool     { return a.Cmp(b) >= 0 }
func BigLess(a *big.Int, b *big.Int) bool    { return a.Cmp(b) == -1 }
func BigGreater(a *big.Int, b *big.Int) bool { return a.Cmp(b) == 1 }

func StringLTE(s string, b *big.Int) (bool, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return false, err
	}
	return BigLTE(a, b), nil
}

func StringBigAdd(s string, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigAdd(a, b)
	return BigIntToString(c), nil
}

func StringSub(s string, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigSub(a, b)
	return BigIntToString(c), nil
}

func StringToBigInt(s string) (*big.Int, ErrorI) {
	b := big.Int{}
	i, ok := b.SetString(s, 10)
	if !ok {
		return nil, errStringToBigInt()
	}
	return i, nil
}

func BigIntToString(b *big.Int) string {
	return b.Text(10)
}

func CopyBytes(bz []byte) (dst []byte) {
	dst = make([]byte, len(bz))
	copy(dst, bz)
	return
}
