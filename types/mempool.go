package types

type Mempool interface {
	Contains(hash string) bool
	AddTransaction(tx []byte, fee string) (recheck bool, err ErrorI)
	DeleteTransaction(tx []byte) ErrorI

	Clear()
	Size() int
	TxsBytes() int
	Iterator() IteratorI
}
