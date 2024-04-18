package lib

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddTransactionFeeOrdering(t *testing.T) {
	mempool := NewMempool(DefaultMempoolConfig())
	recheck, err := mempool.AddTransaction([]byte("b"), "1000")
	require.NoError(t, err)
	require.False(t, recheck)
	recheck, err = mempool.AddTransaction([]byte("c"), "1000")
	require.NoError(t, err)
	require.False(t, recheck)
	recheck, err = mempool.AddTransaction([]byte("a"), "1001")
	require.NoError(t, err)
	require.True(t, recheck) // recheck on non-append insert
	recheck, err = mempool.AddTransaction([]byte("e"), "1")
	require.NoError(t, err)
	require.False(t, recheck)
	recheck, err = mempool.AddTransaction([]byte("d"), "1000")
	require.NoError(t, err)
	require.True(t, recheck) // recheck on non-append insert
	it := mempool.Iterator()
	defer it.Close()
	result := ""
	for ; it.Valid(); it.Next() {
		result += string(it.Key())
	}
	require.Equal(t, "abcde", result)
}
