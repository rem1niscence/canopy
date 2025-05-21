package store

import (
	"reflect"
	"unsafe"

	"github.com/dgraph-io/badger/v4"
)

func getMeta(e *badger.Item) (value byte) {
	v := reflect.ValueOf(e).Elem()
	f := v.FieldByName(badgerMetaFieldName)
	ptr := unsafe.Pointer(f.UnsafeAddr())
	return *(*byte)(ptr)
}
