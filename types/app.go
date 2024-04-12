package types

type App interface {
	LatestHeight() uint64
	HandleTransaction(tx []byte) ErrorI
	GetBeginBlockParams() *BeginBlockParams
	ProduceCandidateBlock(badProposers, doubleSigners [][]byte) (*Block, ErrorI)
	CheckCandidateBlock(candidate *Block, evidence *ByzantineEvidence) (err ErrorI)
	CommitBlock(block *QuorumCertificate) ErrorI
	GetBlockAndCertificate(height uint64) (*QuorumCertificate, ErrorI)
	GetBeginStateValSet(height uint64) (*ValidatorSet, ErrorI)
	EvidenceExists(e *DoubleSignEvidence) (bool, ErrorI)
	GetProducerPubKeys() [][]byte
}

/*
	TODO
		- add n-1 QC to the block structure
		-
*/
