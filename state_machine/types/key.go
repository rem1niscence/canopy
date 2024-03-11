package types

import (
	"fmt"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/types"
)

var (
	accountPrefix = []byte{0x01}
	poolPrefix    = []byte{0x02}
	valPrefix     = []byte{0x03}
	unstakePrefix = []byte{0x04}
	pausedPrefix  = []byte{0x05}
	paramsPrefix  = []byte{0x06}
)

func KeyForAccount(address crypto.AddressI) []byte   { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(name []byte) []byte                  { return append(poolPrefix, name...) }
func KeyForValidator(address crypto.AddressI) []byte { return append(valPrefix, address.Bytes()...) }
func KeyForParams(s string) []byte                   { return append(paramsPrefix, []byte(prefixForParamSpace(s))...) }
func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(append(unstakePrefix, formatHeight(height)...), address.Bytes()...)
}
func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(append(pausedPrefix, formatHeight(maxPausedHeight)...), address.Bytes()...)
}

func AccountPrefix() []byte                { return accountPrefix }
func ValidatorPrefix() []byte              { return valPrefix }
func UnstakingPrefix(height uint64) []byte { return append(unstakePrefix, formatHeight(height)...) }
func PausedPrefix(height uint64) []byte    { return append(pausedPrefix, formatHeight(height)...) }

func AddressFromKey(k []byte) (crypto.AddressI, types.ErrorI) {
	n := len(k)
	if n <= crypto.AddressSize {
		return nil, ErrInvalidAddressKey(k)
	}
	return crypto.NewAddressFromBytes(k[n-crypto.AddressSize:]), nil
}

func formatHeight(height uint64) []byte {
	return []byte(fmt.Sprintf("/%d/", height))
}
