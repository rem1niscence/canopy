package controller

import (
	"bytes"
	"math/rand"
	"time"

	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
)

// ProduceProposal() uses the associated `plugin` to create a Proposal with the candidate block and the `bft` to populate the byzantine evidence
func (c *Controller) ProduceProposal(evidence *bft.ByzantineEvidence, vdf *crypto.VDF) (blk []byte, results *lib.CertificateResult, err lib.ErrorI) {
	// configure the FSM in 'consensus mode' for validator proposals
	resetProposalConfig := c.SetFSMInConsensusModeForProposals()
	// once done proposing, 'reset' the proposal mode back to default to 'accept all'
	defer func() { resetProposalConfig(); c.FSM.Reset() }()
	// load the previous quorum height quorum certificate from the indexer
	lastQC, err := c.FSM.LoadCertificateHashesOnly(c.FSM.Height() - 1)
	if err != nil {
		return
	}
	// re-validate all transactions in the mempool as a 'double check' to ensure all transactions being proposed are valid
	c.Mempool.checkMempool()
	// get the maximum possible size of the block as defined by the governance parameters of the state machine
	maxBlockSize, err := c.FSM.GetMaxBlockSize()
	if err != nil {
		// return error
		return
	}
	// get the maximum amount of transactions allowed or available in the mempool
	transactions, _ := c.Mempool.GetTransactions(maxBlockSize - lib.MaxBlockHeaderSize)
	// validate the verifiable delay function from the bft module
	if vdf != nil {
		// if the verifiable delay function is NOT valid for using the last block hash
		if !crypto.VerifyVDF(lastQC.BlockHash, vdf.Output, vdf.Proof, int(vdf.Iterations)) {
			// nullify the bad VDF
			vdf = nil
			// log the issue but still continue with the proposal
			c.log.Error(lib.ErrInvalidVDF().Error())
		}
	}
	// create the actual block structure
	block := &lib.Block{
		BlockHeader:  &lib.BlockHeader{Time: uint64(time.Now().UnixMicro()), ProposerAddress: c.Address, LastQuorumCertificate: lastQC, Vdf: vdf},
		Transactions: transactions,
	}
	// capture the tentative block result using a new object reference
	blockResult := new(lib.BlockResult)
	// apply the block against the state machine and populate the resulting merkle `roots` in the block header
	block.BlockHeader, blockResult.Transactions, err = c.FSM.ApplyBlock(block)
	if err != nil {
		// return error
		return
	}
	// convert the block reference to bytes
	blk, err = lib.Marshal(block)
	if err != nil {
		// return error
		return
	}
	// update the 'block results' with the newly created header
	blockResult.BlockHeader = block.BlockHeader
	// create a new certificate results (includes reward recipients, slash recipients, swap commands, etc)
	results = c.NewCertificateResults(block, blockResult, evidence)
	// exit
	return
}

// ValidateProposal() fully validates the proposal and resets back to begin block state
func (c *Controller) ValidateProposal(qc *lib.QuorumCertificate, evidence *bft.ByzantineEvidence) (err lib.ErrorI) {
	// the base chain has specific logic to approve or reject proposals
	reset := c.SetFSMInConsensusModeForProposals()
	defer func() { reset(); c.FSM.Reset() }()
	// load the store (db) object
	storeI := c.FSM.Store().(lib.StoreI)
	// perform stateless validations on the block and evidence
	block, err := c.ValidateProposalBasic(qc, evidence)
	if err != nil {
		return err
	}
	// update the LastQuorumCertificate in the store
	//- this is to ensure that each node has the same COMMIT QC for the last block as multiple valid versions (a few less signatures) of the same QC could exist
	// NOTE: this should come before ApplyAndValidateBlock in order to have the proposers 'LastQc' which is used for distributing rewards
	if block.BlockHeader.Height != 1 {
		c.log.Debugf("Indexing last quorum certificate for height %d", block.BlockHeader.LastQuorumCertificate.Header.Height)
		if err = storeI.IndexQC(block.BlockHeader.LastQuorumCertificate); err != nil {
			return err
		}
	}
	// play the block against the state machine
	blockResult, err := c.ApplyAndValidateBlock(block, false)
	if err != nil {
		return
	}
	// create a comparable certificate results (includes reward recipients, slash recipients, swap commands, etc)
	compareResults := c.NewCertificateResults(block, blockResult, evidence)
	// ensure generated the same results
	if !qc.Results.Equals(compareResults) {
		return types.ErrMismatchCertResults()
	}
	return
}

// CommitCertificate used after consensus decides on a block
// - applies block against the fsm
// - indexes the block and its transactions
// - removes block transactions from mempool
// - re-checks all transactions in mempool
// - atomically writes all to the underlying db
// - sets up the app for the next height
func (c *Controller) CommitCertificate(qc *lib.QuorumCertificate) lib.ErrorI {
	// reset the store once this code finishes
	// NOTE: if code execution gets to `store.Commit()` - this will effectively be a noop
	defer func() { c.FSM.Reset() }()
	c.log.Debugf("TryCommit block %s", lib.BytesToString(qc.ResultsHash))
	// load the store (db) object
	storeI := c.FSM.Store().(lib.StoreI)
	// convert the proposal.block (bytes) into Block structure
	blk := new(lib.Block)
	if err := lib.Unmarshal(qc.Block, blk); err != nil {
		return err
	}
	// update the LastQuorumCertificate in the store
	//- this is to ensure that each node has the same COMMIT QC for the last block as multiple valid versions (a few less signatures) of the same QC could exist
	// NOTE: this should come before ApplyAndValidateBlock in order to have the proposers 'LastQc' which is used for distributing rewards
	if blk.BlockHeader.Height != 1 {
		c.log.Debugf("Indexing last quorum certificate for height %d", blk.BlockHeader.LastQuorumCertificate.Header.Height)
		if err := storeI.IndexQC(blk.BlockHeader.LastQuorumCertificate); err != nil {
			return err
		}
	}
	// apply the block against the state machine
	blockResult, err := c.ApplyAndValidateBlock(blk, true)
	if err != nil {
		return err
	}
	// index the quorum certificate in the store
	c.log.Debugf("Indexing quorum certificate for height %d", qc.Header.Height)
	if err = storeI.IndexQC(qc); err != nil {
		return err
	}
	// index the block in the store
	c.log.Debugf("Indexing block %d", blk.BlockHeader.Height)
	if err = storeI.IndexBlock(blockResult); err != nil {
		return err
	}
	// delete the block transactions in the mempool
	for _, tx := range blk.Transactions {
		c.log.Debugf("Tx %s was included in a block so removing from mempool", crypto.HashString(tx))
		c.Mempool.DeleteTransaction(tx)
	}
	// rescan mempool to ensure validity of all transactions
	c.log.Debug("Checking mempool for newly invalid transactions")
	c.Mempool.checkMempool()
	// parse committed block for straw polls
	c.FSM.ParsePollTransactions(blockResult)
	// atomically write this to the store
	c.log.Debug("Committing to store")
	if _, err = storeI.Commit(); err != nil {
		return err
	}
	c.log.Infof("Committed block %s at H:%d 🔒", lib.BytesToString(qc.ResultsHash), blk.BlockHeader.Height)
	c.log.Debug("Setting up FSM for next height")
	c.FSM, err = fsm.New(c.Config, storeI, c.log)
	if err != nil {
		return err
	}
	c.log.Debug("Setting up Mempool for next height")
	if c.Mempool.FSM, err = c.FSM.Copy(); err != nil {
		return err
	}
	if err = c.Mempool.FSM.BeginBlock(); err != nil {
		return err
	}
	c.log.Debug("Commit done")
	return nil
}

// ApplyAndValidateBlock() plays the block against the State Machine and
// compares the block header against a results from the state machine
func (c *Controller) ApplyAndValidateBlock(b *lib.Block, commit bool) (*lib.BlockResult, lib.ErrorI) {
	// basic structural checks on the block
	if err := b.Check(c.Config.NetworkID, c.Config.ChainId); err != nil {
		return nil, err
	}
	// define convenience variables
	blockHash, blockHeight := lib.BytesToString(b.BlockHeader.Hash), b.BlockHeader.Height
	// apply the block against the state machine
	c.log.Debugf("Applying block %s for height %d", blockHash, blockHeight)
	header, txResults, err := c.FSM.ApplyBlock(b)
	if err != nil {
		return nil, err
	}
	// compare the resulting header against the block header (should be identical)
	c.log.Debugf("Validating block header %s for height %d", blockHash, blockHeight)
	if err = c.CompareBlockHeaders(b.BlockHeader, header, commit); err != nil {
		return nil, err
	}
	// return a valid block result
	c.log.Infof("Block %s with %d txs is valid for height %d ✅ ", blockHash, len(b.Transactions), blockHeight)
	return &lib.BlockResult{
		BlockHeader:  b.BlockHeader,
		Transactions: txResults,
	}, nil
}

// CompareBlockHeaders() compares two block headers for equality and validates the last quorum certificate and block time
func (c *Controller) CompareBlockHeaders(candidate *lib.BlockHeader, compare *lib.BlockHeader, commit bool) lib.ErrorI {
	// compare the block headers for equality
	hash, e := compare.SetHash()
	if e != nil {
		return e
	}
	if !bytes.Equal(hash, candidate.Hash) {
		return lib.ErrUnequalBlockHash()
	}
	// validate the last quorum certificate of the block header
	// by using the historical committee
	if candidate.Height > 2 {
		lastCertificate, err := c.FSM.LoadCertificateHashesOnly(candidate.Height - 1)
		if err != nil {
			return err
		}
		// validate the expected QC hash
		if candidate.LastQuorumCertificate == nil ||
			candidate.LastQuorumCertificate.Header == nil ||
			!bytes.Equal(candidate.LastQuorumCertificate.BlockHash, lastCertificate.BlockHash) ||
			!bytes.Equal(candidate.LastQuorumCertificate.ResultsHash, lastCertificate.ResultsHash) {
			return lib.ErrInvalidLastQuorumCertificate()
		}
		// load the committee for the last qc
		committeeHeight := candidate.LastQuorumCertificate.Header.RootHeight
		vs, err := c.LoadCommittee(committeeHeight)
		if err != nil {
			return err
		}
		// check the last QC
		isPartialQC, err := candidate.LastQuorumCertificate.Check(vs, 0, &lib.View{
			Height:     candidate.Height - 1,
			RootHeight: committeeHeight,
			NetworkId:  c.Config.NetworkID,
			ChainId:    c.Config.ChainId,
		}, true)
		if err != nil {
			return err
		}
		// ensure is a full +2/3rd maj QC
		if isPartialQC {
			return lib.ErrNoMaj23()
		}
	}
	// validate VDF
	if candidate.Vdf != nil {
		// if syncing or committing - only verify the vdf randomly since this randomness is pseudo-non-deterministic (among nodes) this holds
		// similar security guarantees but lowers the computational requirements at a per-node basis
		if !commit || rand.Intn(100) == 0 {
			if !crypto.VerifyVDF(candidate.LastQuorumCertificate.BlockHash, candidate.Vdf.Output, candidate.Vdf.Proof, int(candidate.Vdf.Iterations)) {
				return lib.ErrInvalidVDF()
			}
		}
	}
	return nil
}

// ValidateProposalBasic() performs basic structural validates the proposal and resets back to begin block state
func (c *Controller) ValidateProposalBasic(qc *lib.QuorumCertificate, evidence *bft.ByzantineEvidence) (blk *lib.Block, err lib.ErrorI) {
	// validate the byzantine evidence portion of the proposal (bft is canopy controlled)
	if err = c.Consensus.ValidateByzantineEvidence(qc.Results.SlashRecipients, evidence); err != nil {
		return nil, err
	}
	// ensure the block is not empty
	if qc.Block == nil {
		return nil, lib.ErrNilBlock()
	}
	// ensure the results aren't empty
	if qc.Results == nil && qc.Results.RewardRecipients != nil {
		return nil, lib.ErrNilCertResults()
	}
	// convert the block bytes into a Canopy block structure
	blk = new(lib.Block)
	if err = lib.Unmarshal(qc.Block, blk); err != nil {
		return
	}
	// basic structural validations of the block
	if err = blk.Check(c.Config.NetworkID, c.Config.ChainId); err != nil {
		return
	}
	// ensure the Proposal.BlockHash corresponds to the actual hash of the block
	blockHash, e := blk.Hash()
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(qc.BlockHash, blockHash) {
		return nil, lib.ErrMismatchHeaderBlockHash()
	}
	return
}

// CheckPeerQc() performs a basic checks on a block received from a peer and returns if the self is out of sync
func (c *Controller) CheckPeerQC(maxHeight uint64, qc *lib.QuorumCertificate) (stillCatchingUp bool, err lib.ErrorI) {
	// convert the proposal block bytes into a block structure
	blk := new(lib.Block)
	if err = lib.Unmarshal(qc.Block, blk); err != nil {
		return
	}
	// ensure the unmarshalled block is not nil
	if blk == nil || blk.BlockHeader == nil {
		return false, lib.ErrNilBlock()
	}
	// get the hash from the block
	hash, err := blk.Hash()
	if err != nil {
		return false, err
	}
	// ensure the Proposal.BlockHash corresponds to the actual hash of the block
	if !bytes.Equal(qc.BlockHash, hash) {
		err = lib.ErrMismatchQCBlockHash()
		return
	}
	// check the height of the block
	h := c.FSM.Height()
	// don't accept any blocks below the local height
	if h > blk.BlockHeader.Height {
		err = lib.ErrWrongMaxHeight()
		return
	}
	// don't accept any blocks greater than 'max height'
	if blk.BlockHeader.Height > maxHeight {
		err = lib.ErrWrongMaxHeight()
		return
	}
	// height of this block is not local height, the node is still catching up
	stillCatchingUp = h != blk.BlockHeader.Height
	return
}

// SetFSMInConsensusModeForProposals() is how the Validator is configured for `base chain` specific parameter upgrades
func (c *Controller) SetFSMInConsensusModeForProposals() (reset func()) {
	if c.Consensus.GetRound() < 3 {
		// if the node is not having 'consensus issues' refer to the approve list
		c.FSM.SetProposalVoteConfig(types.GovProposalVoteConfig_APPROVE_LIST)
	} else {
		// if the node is exhibiting 'chain halt' like behavior, reject all proposals
		c.FSM.SetProposalVoteConfig(types.GovProposalVoteConfig_REJECT_ALL)
	}
	// a callback that resets the configuration back to default
	reset = func() {
		// the default is to accept all except in 'Consensus mode'
		c.FSM.SetProposalVoteConfig(types.AcceptAllProposals)
	}
	return
}
