package types

type App interface {
	ProduceCandidateBlock() (*Block, ErrorI)
	CheckCandidateBlock(candidate *Block) (err ErrorI)
	CommitBlock(block *Block, params *BeginBlockParams) ErrorI
}
