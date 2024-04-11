package types

type App interface {
	LatestHeight() uint64
	GetBeginBlockParams() *BeginBlockParams
	ProduceCandidateBlock(badProposers, doubleSigners [][]byte) (*Block, ErrorI)
	CheckCandidateBlock(candidate *Block) (err ErrorI)
	CommitBlock(block *QuorumCertificate) ErrorI
	GetBlockAndCertificate(height uint64) (*QuorumCertificate, ErrorI)
	GetBeginStateValSet(height uint64) (*ValidatorSet, ErrorI)
	EvidenceExists(e *DoubleSignEvidence) (bool, ErrorI)
}

/*
	TODO
		- add n-1 QC to the block structure
		-
*/
