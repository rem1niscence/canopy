package smt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test(t *testing.T) {
	smn, smv := NewSimpleMap(), NewSimpleMap()
	lazy := NewSMT(smn, sha256.New())
	smt := &SMTWithStorage{SparseMerkleTree: lazy, preimages: smv}

	require.NoError(t, smt.Update([]byte("testKey"), []byte("testValue")))
	require.NoError(t, smt.Update([]byte("foo"), []byte("testValue")))
	fmt.Println(smn)
	smt.Commit()
	fmt.Println(smn)
	value, err := smt.GetValue([]byte("testKey"))
	fmt.Println(string(value), err)
	value, err = lazy.Get([]byte("testKey"))
	fmt.Println(hex.EncodeToString(value))
	fmt.Println(hex.EncodeToString(smt.base().digestValue([]byte("testValue"))))
}
