package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math"
	"strconv"
)

var ReservedIDs = []uint64{
	lib.CanopyCommitteeId,
	lib.DAOPoolID,
}

const EscrowPoolAddend = math.MaxUint16

var (
	accountPrefix        = []byte{1}  // store key prefix for accounts
	poolPrefix           = []byte{2}  // store key prefix for pools
	validatorPrefix      = []byte{3}  // store key prefix for validators
	committeePrefix      = []byte{4}  // store key prefix for validators in committees
	unstakePrefix        = []byte{5}  // store key prefix for validators currently unstaking
	pausedPrefix         = []byte{6}  // store key prefix for validators currently paused
	paramsPrefix         = []byte{7}  // store key prefix for governance parameters
	nonSignerPrefix      = []byte{8}  // store key prefix for validators who have missed signing QCs
	lastProposersPrefix  = []byte{9}  // store key prefix for the last proposers
	supplyPrefix         = []byte{10} // store key prefix for the supply count
	delegatePrefix       = []byte{11} // store key prefix for the validators who are delegating for committees
	committeesDataPrefix = []byte{12} // store key prefix for Quorum Certificate proposals before they are paid
	orderBookPrefix      = []byte{13} // store key prefix for 'sell orders' before they are bid on
)

func KeyForAccount(address crypto.AddressI) []byte { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(n uint64) []byte                   { return append(poolPrefix, formatUint64(n)...) }
func KeyForValidator(addr crypto.AddressI) []byte  { return append(validatorPrefix, addr.Bytes()...) }
func KeyForParams(s string) []byte                 { return append(paramsPrefix, []byte(prefixForParamSpace(s))...) }
func KeyForNonSigner(a []byte) []byte              { return append(nonSignerPrefix, a...) }
func KeyForOrderBook(id uint64) []byte             { return append(orderBookPrefix, formatUint64(id)...) }
func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(UnstakingPrefix(height), address.Bytes()...)
}
func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(PausedPrefix(maxPausedHeight), address.Bytes()...)
}
func KeyForCommittee(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, stake)
	return append(append(CommitteePrefix(committeeID), b...), addr.Bytes()...)
}
func KeyForDelegate(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, stake)
	return append(append(DelegatePrefix(committeeID), b...), addr.Bytes()...)
}
func AccountPrefix() []byte                { return accountPrefix }
func PoolPrefix() []byte                   { return poolPrefix }
func SupplyPrefix() []byte                 { return supplyPrefix }
func ValidatorPrefix() []byte              { return validatorPrefix }
func NonSignerPrefix() []byte              { return nonSignerPrefix }
func UnstakingPrefix(height uint64) []byte { return append(unstakePrefix, formatUint64(height)...) }
func PausedPrefix(height uint64) []byte    { return append(pausedPrefix, formatUint64(height)...) }
func LastProposersPrefix() []byte          { return lastProposersPrefix }
func CommitteePrefix(id uint64) []byte     { return append(committeePrefix, formatUint64(id)...) }
func DelegatePrefix(id uint64) []byte      { return append(delegatePrefix, formatUint64(id)...) }
func OrderBookPrefix() []byte              { return orderBookPrefix }
func CommitteesDataPrefix() []byte         { return committeesDataPrefix }
func AddressFromKey(k []byte) (crypto.AddressI, lib.ErrorI) {
	n := len(k)
	if n <= crypto.AddressSize {
		return nil, ErrInvalidKey(k)
	}
	return crypto.NewAddressFromBytes(k[n-crypto.AddressSize:]), nil
}

func IdFromKey(k []byte) (uint64, lib.ErrorI) {
	if bytes.Count(k, []byte("/")) != 2 {
		return 0, ErrInvalidKey(k)
	}
	id, err := strconv.Atoi(string(bytes.Split(k, []byte("/"))[1]))
	if err != nil {
		return 0, ErrInvalidKey(k)
	}
	return uint64(id), nil
}

func formatUint64(u uint64) []byte {
	return []byte(fmt.Sprintf("/%d/", u))
}
