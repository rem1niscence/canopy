package types

import (
	"fmt"
	"github.com/ginchuco/ginchu/crypto"
)

var (
	accountPrefix = []byte{0x01}
	poolPrefix    = []byte{0x02}
	valPrefix     = []byte{0x03}
	unstakePrefix = []byte{0x04}
	paramsPrefix  = []byte{0x05}
)

func KeyForAccount(address crypto.AddressI) []byte   { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(name string) []byte                  { return append(poolPrefix, []byte(name)...) }
func KeyForValidator(address crypto.AddressI) []byte { return append(valPrefix, address.Bytes()...) }
func KeyForParams(s string) []byte                   { return append(paramsPrefix, []byte(prefixForParamSpace(s))...) }
func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(append(unstakePrefix, formatHeight(height)...), address.Bytes()...)
}

func AccountPrefix() []byte                { return accountPrefix }
func ValidatorPrefix() []byte              { return valPrefix }
func UnstakingPrefix(height uint64) []byte { return append(unstakePrefix, formatHeight(height)...) }

func formatHeight(height uint64) []byte {
	return []byte(fmt.Sprintf("/%d/", height))
}
