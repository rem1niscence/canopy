package controller

import (
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/plugin"
	"time"
)

// HandleTransaction() accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Controller) HandleTransaction(committeeID uint64, tx []byte) lib.ErrorI {
	// lock the controller for thread safety
	c.Lock()
	defer c.Unlock()
	// get the chain
	chain, err := c.GetChain(committeeID)
	if err != nil {
		return err
	}
	// handle the transaction
	return chain.Plugin.HandleTx(tx)
}

// ValidateCertificate() fully validates the proposal and resets back to begin block state
func (c *Controller) ValidateCertificate(committeeID uint64, qc *lib.QuorumCertificate, evidence *bft.ByzantineEvidence) (err lib.ErrorI) {
	// the base chain has specific logic to approve or reject proposals
	if committeeID == lib.CanopyCommitteeId {
		reset := c.ValidatorProposalConfig()
		defer func() { reset() }()
	}
	chain, err := c.GetChain(committeeID)
	if err != nil {
		return
	}
	// validate the byzantine evidence portion of the proposal (bft is canopy controlled)
	if err = chain.Consensus.ValidateByzantineEvidence(qc.Results.SlashRecipients, evidence); err != nil {
		return err
	}
	// validate the rest of the proposal (block / reward recipients may only be determined and/or interpreted by the plugin)
	return chain.Plugin.ValidateCertificate(c.CanopyFSM().Height(), qc)
}

// GetChain() returns the chain object for a specific committeeID, if not supported - then error
func (c *Controller) GetChain(committeeID uint64) (*Chain, lib.ErrorI) {
	// get the chain from the map
	chain, ok := c.Chains[committeeID]
	if !ok {
		return nil, lib.ErrWrongCommitteeID()
	}
	// return the chain
	return chain, nil
}

// ProduceProposal() uses the associated `plugin` to create a Proposal with the candidate block and the `bft` to populate the byzantine evidence
func (c *Controller) ProduceProposal(committeeID uint64, be *bft.ByzantineEvidence, vdf *crypto.VDF) (block []byte, results *lib.CertificateResult, err lib.ErrorI) {
	if committeeID == lib.CanopyCommitteeId {
		reset := c.ValidatorProposalConfig()
		defer func() { reset() }()
	}
	chain, err := c.GetChain(committeeID)
	if err != nil {
		return
	}
	// use the plugin to make the 'proposal'
	block, rewardRecipients, err := chain.Plugin.ProduceProposal(vdf)
	if err != nil {
		return
	}
	results = &lib.CertificateResult{
		RewardRecipients: rewardRecipients,
		SlashRecipients:  new(lib.SlashRecipients),
	}
	// use the bft object to fill in the Byzantine Evidence
	results.SlashRecipients.DoubleSigners, err = chain.Consensus.ProcessDSE(be.DSE.Evidence...)
	if err != nil {
		c.log.Warn(err.Error()) // still produce proposal
	}
	return
}

// ResetBFTCallback() is the function the `plugin` calls to let the controller's bft know to reset and start over
func (c *Controller) ResetBFTCallback(committeeID uint64) {
	var err lib.ErrorI
	// lock the controller for thread safety
	c.Lock()
	defer c.Unlock()
	// for each chain
	chain, ok := c.Chains[committeeID]
	if !ok {
		c.log.Errorf("failed retrieving bft object when trigger occurred %s", lib.ErrWrongCommitteeID())
		return
	}
	// update the canopy committee members
	if chain.Consensus.ValidatorSet, err = c.CanopyFSM().GetCommitteeMembers(committeeID); err != nil {
		c.log.Errorf("failed retrieving committee when trigger occurred %s", err.Error())
		return
	}
	// reset the consensus for that particular chain
	chain.Consensus.ResetBFTChan() <- bft.ResetBFT{
		ProcessTime: time.Duration(0),
	}
}

// CANOPY (BASE CHAIN) SPECIFIC FUNCTIONALITY BELOW

// ValidatorProposalConfig() is how the Validator is configured for `base chain` specific parameter upgrades
func (c *Controller) ValidatorProposalConfig() (reset func()) {
	if c.Chains[lib.CanopyCommitteeId].Consensus.GetRound() < 3 {
		// if the node is not having 'consensus issues' refer to the approve list
		c.CanopyFSM().SetProposalVoteConfig(types.GovProposalVoteConfig_APPROVE_LIST)
	} else {
		// if the node is exhibiting 'chain halt' like behavior, reject all proposals
		c.CanopyFSM().SetProposalVoteConfig(types.GovProposalVoteConfig_REJECT_ALL)
	}
	// a callback that resets the configuration back to default
	reset = func() {
		// the default is to accept all except in 'Consensus mode'
		c.CanopyFSM().SetProposalVoteConfig(types.AcceptAllProposals)
	}
	return
}

// GetPendingPage() returns a page of unconfirmed mempool transactions
func (c *Controller) GetPendingPage(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	// lock the controller for thread safety
	c.Lock()
	defer c.Unlock()
	// get the canopy chain from the controller map
	chain, err := c.GetChain(lib.CanopyCommitteeId)
	if err != nil {
		return nil, err
	}
	// return the pending page for rpc from the canpy plugin
	return chain.Plugin.(plugin.CanopyPlugin).PendingPageForRPC(p)
}
