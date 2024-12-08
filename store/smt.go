package store

import (
	"bytes"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
)

// =====================================================
// SMT: An optimized sparse Merkle tree
// =====================================================
//
// 1. Any leaf nodes without values are set to nil. A parent node is also nil
//    if both children are nil.
// 2. If a parent has exactly one non-nil child, replace the parent with
//    the non-nil child.
// 3. A tree always starts with two children: (0x0...) and (FxF...), and a Root.
//
// -----------------------------------------------------
// Variables:
// -----------------------------------------------------
// - Target: The node (or its ID) being inserted or deleted.
// - Current: The currently selected node (or its ID).
// - GCP: The greatest common prefix between the Target and Current's keys,
//   representing the shared path.
// - Path_Bit: The currently selected bit.
//
// -----------------------------------------------------
// 1) Traversal: Navigate the tree downward to locate the target or its
// closest position.
// -----------------------------------------------------
// - Start at the root: (Current = Root) and (gcp = ∅).
// - LOOP:
//   1. Calculate the path bit:
//      - path_bit is the first bit in Target.Key after gcp.
//   2. Traverse to the next node:
//      - If path_bit = 0, move left: Current = Current.LeftChild.
//      - If path_bit = 1, move right: Current = Current.RightChild.
//   3. Calculate the gcp:
//      - gcp is the greatest common prefix between the Target.Key and
//        the Current.Key.
//   4. Exit the loop if:
//      - Current is empty, OR
//      - Current.key is not gcp, OR
//      - Target is gcp.
//
// -----------------------------------------------------
// 2.a) Upsert: Insert or update the target node.
// -----------------------------------------------------
// - If gcp = Target_Key:
//   - Set Current = Target and skip to step 5.
// - Create a new node:
//   - new_node = create(key = gcp).
// - Replace the reference to Current in its parent with new_node:
//   - If Current is parent.LeftChild: parent.LeftChild = new_node.
//   - Else: parent.RightChild = new_node.
// - Set Current and Target as children of new_node:
//   - If path_bit = 0: new_node.Children = {Target, Current}.
//   - If path_bit = 1: new_node.Children = {Current, Target}.
// - Point to parent for ReHash:
//   - Current = new_node.parent.
//
// -----------------------------------------------------
// 2.b) Delete: Remove the target node.
// -----------------------------------------------------
// - If gcp = Target_Key:
//   - Delete Current.
//   - Else: Exit.
// - Replace the reference to parent in the grandparent with
//   Current.sibling:
//   - If parent is grandparent.LeftChild: grandparent.LeftChild =
//     Current.sibling.
//   - Else: grandparent.RightChild = Current.sibling.
// - Delete the parent node:
//   - delete(Current.parent).
// - Point to grandparent for ReHash:
//   - Current = grandparent.
//
// -----------------------------------------------------
// 3) ReHash: Recalculate hashes from the current node upwards.
// -----------------------------------------------------
// - LOOP:
//   1. Update Current.value:
//      Current.value = Hash(Current.LeftChild.value,
//                          Current.RightChild.value).
//   2. If Current is the root, finish.
//   3. Otherwise, move up:
//      Current = Current.parent.
//
// =====================================================

const DefaultKeyBitLength = crypto.HashSize * 8

type SMT struct {
	// store: an abstraction of the database where the tree is being stored
	store lib.RWStoreI
	// root: the root node
	root *Node
	// keyBitLength: the depth of the tree, once set it cannot be changed for a protocol
	keyBitLength int

	// OpData: data for each operation
	OpData
}

type OpData struct {
	// gcp: The greatest common prefix between the Target and Current’s keys, representing the shared path
	gcp []byte
	// bitPos: The bit position of the bit after the gcp in target_key
	bitPos int
	// pathBit: The bit at bitPos
	pathBit int
	// target: the node that is being added or deleted (or its ID)
	target *Node
	// current: the current selected nod
	current *Node
	// traversed: a descending list of traversed nodes from root to parent of current
	traversed *NodeList
}

// NewSMT() creates a new abstraction of the SMT object
func NewSMT(rootKey []byte, store lib.RWStoreI) *SMT {
	smt := &SMT{
		store:        store,
		keyBitLength: DefaultKeyBitLength,
	}
	// get the root from the store
	smt.root = smt.getNode(rootKey)
	// if the root is empty, initialize with min and max node
	if smt.root == nil {
		smt.InitializeTree(rootKey)
	}
	return smt
}

// InitializeTree() ensures the tree always has a root with two children
// this allows the logic to be without root edge cases for insert and delete
func (s *SMT) InitializeTree(rootKey []byte) {
	// create a min and max node, this enables no edge cases for root
	minNode := &Node{Key: crypto.MinHash[:s.keyBitLength], Value: crypto.MinHash}
	maxNode := &Node{Key: crypto.MaxHash[:s.keyBitLength], Value: crypto.MaxHash}
	// set min and max node in the database
	s.setNode(minNode)
	s.setNode(maxNode)
	// update root
	s.root = &Node{
		Key:           rootKey,
		RightChildKey: maxNode.Key,
		LeftChildKey:  minNode.Key,
	}
	// update the root's value
	s.updateParentValue(s.root, minNode)
	// set the root in store
	s.setNode(s.root)
}

// Set: insert or update a target
func (s *SMT) Set(k, v []byte) lib.ErrorI {
	// calculate the key and value to upsert
	s.target = &Node{Key: crypto.Hash(k)[:s.keyBitLength], Value: v}
	// navigates the tree downward
	s.traverse()
	// if gcp != target key then it is an insert not an update
	if !bytes.Equal(s.target.Key, s.gcp) {
		// create a new node (new parent of current and target)
		newParent, newParentKey := &Node{}, append([]byte{}, s.gcp...)
		// get the parent (soon to be grandparent) of current
		oldParent := s.traversed.Parent()
		// replace the reference to Current in its parent with the new parent
		oldParent.replaceChild(s.current.Key, newParentKey)
		// set current and target as children of new parent
		// NOTE: the old parent is now the grandparent of target and current
		switch s.pathBit {
		case 0:
			newParent.setChildren(s.target.Key, s.current.Key)
		case 1:
			newParent.setChildren(s.current.Key, s.target.Key)
		}
		// add new node to traversed list, as it's the new parent for current and target
		// and should come after the grandparent (previously parent)
		s.traversed.Nodes = append(s.traversed.Nodes, newParent)
	}
	// set the node in the database
	s.setNode(s.target)
	// finish with a rehashing of the tree
	return s.rehash()
}

// Delete: removes a target node if exists in the tree
func (s *SMT) Delete(k []byte) lib.ErrorI {
	// calculate the key and value to upsert
	s.target = &Node{Key: crypto.Hash(k)[:s.keyBitLength]}
	// navigates the tree downward
	s.traverse()
	// if gcp != target key then there is no delete because the node does not exist
	if !bytes.Equal(s.target.Key, s.gcp) {
		return nil
	}
	// get the parent and grandparent
	parent, grandparent := s.traversed.Parent(), s.traversed.GrandParent()
	// get the sibling of the target
	sibling := parent.getOtherChild(s.target.Key)
	// replace the parent reference with the sibling in the grandparent
	grandparent.replaceChild(parent.Key, sibling)
	// delete the parent from the database and remove it from the traversal array
	s.delNode(parent.Key)
	s.traversed.Pop()
	// delete the target from the database
	s.delNode(s.target.Key)
	// finish with a rehashing of the tree
	return s.rehash()
}

// traverse: navigates the tree downward to locate the target or its closest position
func (s *SMT) traverse() {
	s.reset()
	// execute main loop
	for {
		// add current to traversed
		s.traversed.Nodes = append(s.traversed.Nodes, s.current)
		// decide to move left or right based on the bit-value of the key
		switch s.pathBit = s.getBit(s.target.Key); s.pathBit {
		case 0: // move down to the left
			s.current.Key = s.current.LeftChildKey
		case 1: // move down to the right
			s.current.Key = s.current.RightChildKey
		}
		// update the greatest common prefix and the bit position based on the new current key
		s.updateGCPAndBitPosition()
		// load current node from the store
		s.current = s.getNode(s.current.Key)
		// exit conditions
		if s.current == nil || !bytes.Equal(s.current.Key, s.gcp) || bytes.Equal(s.target.Key, s.gcp) {
			return // exit loop
		}
	}
}

// rehash() recalculate hashes from the current node upwards
func (s *SMT) rehash() lib.ErrorI {
	// create a convenience variable for the max index of the array
	maxIdx := len(s.traversed.Nodes) - 1
	// iterate the traversed list from end to start
	for i := maxIdx; i >= 0; i++ {
		// child stores the cached from the parent that was traversed
		var child *Node
		// select the parent
		parent := s.traversed.Nodes[i]
		// get a child from the traversed list if possible
		if i != maxIdx {
			child = s.traversed.Nodes[i+1]
		}
		// calculate its new value
		s.updateParentValue(parent, child)
		// set node in the database
		s.setNode(parent)
	}
	return nil
}

// getBit retrieves the bit at a specific bit position from a byte slice
func (s *SMT) getBit(data []byte) int {
	// Ensure the bitPos is within bounds of the byte slice
	if s.bitPos < 0 || s.bitPos >= 8*len(data) {
		panic("bitPos out of bounds")
	}
	// calculate the byte index and the bit index within that byte
	byteIndex := s.bitPos / 8
	bitIndex := 7 - (s.bitPos % 8)
	// get the byte at byte index
	byt := data[byteIndex]
	// use bitwise to retrieve the bit value
	return (byt >> bitIndex) & 1
}

// updateGCPAndBitPosition updates the greatest common bit prefix and position between two byte slices - starting from an existing common prefix
// CONTRACT: len(targetKey) == keyBitLength
func (s *SMT) updateGCPAndBitPosition() {
	// traverse both byte slices bit by bit starting at bit position
	for ; s.bitPos < len(s.current.Key)*8; s.bitPos++ {
		// If the bits match, add to the common prefix
		bit1, bit2 := s.getBit(s.target.Key), s.getBit(s.current.Key)
		if bit1 != bit2 {
			break
		}
		// make sure to add the matching bit as a byte
		if len(s.gcp)*8 <= s.bitPos {
			// Add a new byte if we are starting a new byte in the prefix
			s.gcp = append(s.gcp, 0)
		}
		// calculate the new bit index in the last byte of common prefix using MSB logic
		bitIndex := 7 - (s.bitPos % 8)
		// perform bit shift to position the bit for the last byte of the common prefix
		// example: bit1 = 1 and bitIndex = 2 then shifted = int(b00000100)
		shifted := bit1 << bitIndex
		// apply logical OR to the last byte of the common prefix to set the bit
		s.gcp[len(s.gcp)-1] |= shifted
	}
	return
}

// updateParentValue() updates the value of parent based on its children
func (s *SMT) updateParentValue(parent, child *Node) {
	var rightChild, leftChild *Node
	// if there's a child from the traversed list
	if child != nil {
		// determine if it's the right or left child for the parent
		if bytes.Equal(parent.RightChildKey, child.Key) {
			rightChild = child
		} else {
			leftChild = child
		}
	}
	// concatenate the left and right children values
	rightPlusLeft := append(s.childInput(parent.LeftChildKey, leftChild), s.childInput(parent.RightChildKey, rightChild)...)
	// update the parents value
	parent.Value = crypto.Hash(rightPlusLeft)
}

// childInput() returns key + value of the child, retrieving the node from the db if needed
func (s *SMT) childInput(childKey []byte, child *Node) []byte {
	// if the child is not populated
	if child == nil {
		// get the child from the database
		child = s.getNode(childKey)
	}
	return append(child.Key, child.Value...)
}

// reset() resets data for each operation
func (s *SMT) reset() {
	s.current = s.root
	s.gcp, s.target = nil, nil
	s.pathBit, s.bitPos = 0, 0
	s.traversed = &NodeList{Nodes: make([]*Node, 0)}
}

func (n *Node) bytes() []byte {
	// omit the key when saving to the database
	n.Key = nil
	panic("TODO not implemented")
}

func (s *SMT) setNode(n *Node) {
	panic("TODO not implemented")
}
func (s *SMT) delNode(key []byte) {
	panic("TODO not implemented")
}
func (s *SMT) getNode(key []byte) (n *Node) {
	// add the key to the node structure for convenience
	n.Key = key
	panic("TODO not implemented")
}

// Node represents a single element in a sparse Merkle tree
// It stores the cryptographic hash of data and the structural information
// required to traverse and reconstruct the tree
type Node struct {
	Key []byte
	// Value is the cryptographic hash of the data included in the database
	// the ValueHash is included in the parent hash
	Value []byte
	// RightChildKey is the key for the right child node. Nil means no child
	RightChildKey []byte
	// LeftChildKey is the key for the left child node. Nil means no child
	LeftChildKey []byte
}

// NewNode() constructs a new instance of a Node object
func NewNode(value, leftChild, rightChild []byte) *Node {
	return &Node{
		Value:         value,
		RightChildKey: leftChild,
		LeftChildKey:  rightChild,
	}
}

// setChildren() sets the children of a node in its structure
func (n *Node) setChildren(leftKey, rightKey []byte) {
	n.LeftChildKey, n.RightChildKey = leftKey, rightKey
}

// getOtherChild() returns the sibling for the child key passed
func (n *Node) getOtherChild(childKey []byte) []byte {
	switch {
	case bytes.Equal(n.LeftChildKey, childKey):
		return n.RightChildKey
	case bytes.Equal(n.RightChildKey, childKey):
		return n.LeftChildKey
	}
	panic("no child node was a match for getOtherChild")
}

// replaceChild() replaces the child reference with a new key
func (n *Node) replaceChild(oldKey, newKey []byte) {
	switch {
	case bytes.Equal(n.LeftChildKey, oldKey):
		n.LeftChildKey = newKey
	case bytes.Equal(n.RightChildKey, oldKey):
		n.RightChildKey = newKey
	}
	panic("no child node was replaced")
}

// NodeList defines a list of nodes, used for traversal and merkle proofs
type NodeList struct {
	Nodes []*Node
}

// Parent() returns the parent of the last node traversed (current)
func (n NodeList) Parent() *Node { return n.Nodes[len(n.Nodes)-1] }

// GrandParent() returns the grandparent of the last node traversed (current)
func (n NodeList) GrandParent() *Node { return n.Nodes[len(n.Nodes)-2] }

// Pop() removes the node from the list
func (n NodeList) Pop() { n.Nodes = n.Nodes[:len(n.Nodes)-1] }
