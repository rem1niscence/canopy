package types

type Mempool interface {
	Contains(hash string) bool
	AddTransaction(tx []byte) ErrorI
	DeleteTransaction(tx []byte) ErrorI

	Clear()
	Size() int
	TxsBytes() int
	PopTransaction() (tx []byte, err ErrorI)
}
