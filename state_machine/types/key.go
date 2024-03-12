package types

import (
	"fmt"
	"github.com/ginchuco/ginchu/crypto"
	"github.com/ginchuco/ginchu/types"
	"math/big"
)

var (
	accountPrefix   = []byte{1}
	poolPrefix      = []byte{2}
	validatorPrefix = []byte{3}
	consensusPrefix = []byte{4}
	unstakePrefix   = []byte{5}
	pausedPrefix    = []byte{6}
	paramsPrefix    = []byte{7}
	nonSignerPrefix = []byte{8}
)

func KeyForAccount(address crypto.AddressI) []byte { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(name []byte) []byte                { return append(poolPrefix, name...) }
func KeyForValidator(addr crypto.AddressI) []byte  { return append(validatorPrefix, addr.Bytes()...) }
func KeyForParams(s string) []byte                 { return append(paramsPrefix, []byte(prefixForParamSpace(s))...) }
func KeyForNonSigner(a crypto.AddressI) []byte     { return append(nonSignerPrefix, a.Bytes()...) }

func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(append(unstakePrefix, formatHeight(height)...), address.Bytes()...)
}

func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(append(pausedPrefix, formatHeight(maxPausedHeight)...), address.Bytes()...)
}

func KeyForConsensus(address crypto.AddressI, stake *big.Int) []byte {
	return append(append(consensusPrefix, stake.Bytes()...), address.Bytes()...)
}

func AccountPrefix() []byte                { return accountPrefix }
func ValidatorPrefix() []byte              { return validatorPrefix }
func ConsensusPrefix() []byte              { return consensusPrefix }
func NonSignerPrefix() []byte              { return nonSignerPrefix }
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
