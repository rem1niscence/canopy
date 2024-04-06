package types

import (
	"math/big"
	"strconv"
)

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

func StringReducePercentage(amount string, percent int8) (string, ErrorI) {
	// convert amount to big.Int
	a, err := StringToBigInt(amount)
	if err != nil {
		return "", err
	}
	// convert to a percent float: ex 1 - .05 = .95
	percentFloat := big.NewFloat(float64(1) - float64(percent)/float64(100))
	// amountBig.ToFloat() * percentFloat: ex 100 * .95 = 95 (reduced by 5%)
	result := new(big.Float).Mul(new(big.Float).SetInt(a), percentFloat)
	// truncate to big.Int
	c, _ := result.Int(nil)
	// convert back to string
	return BigIntToString(c), nil
}

func StringBigLTE(s string, b *big.Int) (bool, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return false, err
	}
	return BigLTE(a, b), nil
}

func StringsLess(s, s2 string) (bool, ErrorI) {
	cmp, err := StringsCmp(s, s2)
	if err != nil {
		return false, err
	}
	return cmp == -1, nil
}

func StringsGTE(s, s2 string) (bool, ErrorI) {
	cmp, err := StringsCmp(s, s2)
	if err != nil {
		return false, err
	}
	return cmp >= 0, nil
}

func StringsCmp(s, s2 string) (int, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return 0, err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return 0, err
	}
	return a.Cmp(b), nil
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

func StringAdd(s string, s2 string) (string, ErrorI) {
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

func StringDiv(s, s2 string) (string, ErrorI) {
	a, err := StringToBigInt(s)
	if err != nil {
		return "", err
	}
	b, err := StringToBigInt(s2)
	if err != nil {
		return "", err
	}
	c := BigDiv(a, b)
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

func StringToUint64(s string) uint64 {
	i, _ := strconv.ParseUint(s, 10, 64)
	return i
}

func BigIntToString(b *big.Int) string {
	return b.Text(10)
}

func CopyBytes(bz []byte) (dst []byte) {
	dst = make([]byte, len(bz))
	copy(dst, bz)
	return
}

func MapCopy[M1 ~map[K]V, M2 ~map[K]V, K comparable, V any](dst M1, src M2) {
	for k, v := range src {
		dst[k] = v
	}
}
