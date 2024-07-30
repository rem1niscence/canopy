package types

import (
	"encoding/binary"
	"fmt"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
)

var (
	accountPrefix       = []byte{1}
	poolPrefix          = []byte{2}
	validatorPrefix     = []byte{3}
	consensusPrefix     = []byte{4}
	unstakePrefix       = []byte{5}
	pausedPrefix        = []byte{6}
	paramsPrefix        = []byte{7}
	nonSignerPrefix     = []byte{8}
	doubleSignersPrefix = []byte{9}
	producersPrefix     = []byte{10}
	supplyPrefix        = []byte{11}
)

func KeyForAccount(address crypto.AddressI) []byte { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(n PoolID) []byte                   { return append(poolPrefix, lib.ProtoEnumToBytes(uint32(n))...) }
func KeyForValidator(addr crypto.AddressI) []byte  { return append(validatorPrefix, addr.Bytes()...) }
func KeyForParams(s string) []byte                 { return append(paramsPrefix, []byte(prefixForParamSpace(s))...) }
func KeyForNonSigner(a []byte) []byte              { return append(nonSignerPrefix, a...) }
func KeyForDoubleSigner(h uint64, a []byte) []byte { return append(DoubleSignersPrefix(h), a...) }
func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(append(unstakePrefix, formatHeight(height)...), address.Bytes()...)
}

func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(append(pausedPrefix, formatHeight(maxPausedHeight)...), address.Bytes()...)
}

func KeyForConsensus(address crypto.AddressI, stake uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, stake)
	return append(append(consensusPrefix, b...), address.Bytes()...)
}

func AccountPrefix() []byte                { return accountPrefix }
func PoolPrefix() []byte                   { return poolPrefix }
func SupplyPrefix() []byte                 { return supplyPrefix }
func ValidatorPrefix() []byte              { return validatorPrefix }
func ConsensusPrefix() []byte              { return consensusPrefix }
func NonSignerPrefix() []byte              { return nonSignerPrefix }
func DoubleSignersPrefix(h uint64) []byte  { return append(doubleSignersPrefix, formatHeight(h)...) }
func DoubleSignerEnabledByte() []byte      { return doubleSignersPrefix }
func UnstakingPrefix(height uint64) []byte { return append(unstakePrefix, formatHeight(height)...) }
func PausedPrefix(height uint64) []byte    { return append(pausedPrefix, formatHeight(height)...) }
func ProducersPrefix() []byte              { return producersPrefix }
func AddressFromKey(k []byte) (crypto.AddressI, lib.ErrorI) {
	n := len(k)
	if n <= crypto.AddressSize {
		return nil, ErrInvalidAddressKey(k)
	}
	return crypto.NewAddressFromBytes(k[n-crypto.AddressSize:]), nil
}

func formatHeight(height uint64) []byte {
	return []byte(fmt.Sprintf("/%d/", height))
}
