package types

import "github.com/ginchuco/ginchu/types/crypto"

func MerkleTree(items [][]byte) (root []byte, tree [][]byte, err ErrorI) {
	root, tree, er := crypto.MerkleTree(items)
	if er != nil {
		return nil, nil, ErrMerkleTree(er)
	}
	return
}
