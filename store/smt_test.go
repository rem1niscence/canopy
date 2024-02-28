package store

import (
	"fmt"
	"github.com/ginchuco/ginchu/types"
	"testing"
)

func TestSMT(t *testing.T) {
	ms := NewMemoryStore(types.NewDefaultLogger())
	store := ms.(*Store)
	if err := store.Set([]byte("a"), []byte("a")); err != nil {
		t.Fatal(err)
	}
	store.smt.Commit()
	store.store.Write()
	levelDB := store.store.parent
	it, err := levelDB.Iterator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		fmt.Println(string(it.Key()))
	}
}
