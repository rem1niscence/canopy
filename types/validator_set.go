package types

func (x *ValidatorSet) Root() ([]byte, ErrorI) {
	if x == nil || len(x.ValidatorSet) == 0 {
		return nil, nil
	}
	var bytes [][]byte
	for _, val := range x.ValidatorSet {
		bz, err := Marshal(val)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, bz)
	}
	root, _, err := MerkleTree(bytes)
	return root, err
}
