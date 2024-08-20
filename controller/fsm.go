package controller

import (
	"github.com/ginchuco/ginchu/bft"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/plugin"
	"time"
)

// HandleTransaction accepts or rejects inbound txs based on the mempool state
// - pass through call checking indexer and mempool for duplicate
func (c *Controller) HandleTransaction(committeeID uint64, tx []byte) lib.ErrorI {
	c.Lock()
	defer c.Unlock()
	plug, _, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return err
	}
	return plug.HandleTx(tx)
}

// CheckProposal checks the proposal for errors
func (c *Controller) CheckProposal(committeeID uint64, prop *lib.Proposal) (err lib.ErrorI) {
	committeeHeight := c.FSM.Height()
	_, err = prop.Check(committeeID, committeeHeight)
	return
}

// ValidateProposal fully validates the proposal and resets back to begin block state
func (c *Controller) ValidateProposal(committeeID uint64, p *lib.Proposal, evidence *bft.ByzantineEvidence) (err lib.ErrorI) {
	if committeeID == lib.CANOPY_COMMITTEE_ID {
		reset := c.ValidatorProposalConfig(c.FSM)
		defer func() { reset() }()
	}
	plug, cons, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return
	}
	if err = cons.ValidateByzantineEvidence(p, evidence); err != nil {
		return err
	}
	return plug.ValidateProposal(c.FSM.Height(), p)
}

func (c *Controller) GetPluginAndConsensus(committeeID uint64) (plugin.Plugin, *bft.Consensus, lib.ErrorI) {
	p, ok := c.Plugins[committeeID]
	if !ok {
		return nil, nil, lib.ErrWrongCommitteeID()
	}
	cons, ok := c.Consensus[committeeID]
	if !ok {
		return nil, nil, lib.ErrWrongCommitteeID()
	}
	return p, cons, nil
}

// ProduceProposal uses the mempool and state params to build a proposal block
func (c *Controller) ProduceProposal(committeeID uint64, be *bft.ByzantineEvidence) (*lib.Proposal, lib.ErrorI) {
	if committeeID == lib.CANOPY_COMMITTEE_ID {
		reset := c.ValidatorProposalConfig(c.FSM)
		defer func() { reset() }()
	}
	plug, cons, err := c.GetPluginAndConsensus(committeeID)
	if err != nil {
		return nil, err
	}
	proposal, err := plug.ProduceProposal(c.FSM.Height())
	if err != nil {
		return nil, err
	}
	proposal.Meta.CommitteeHeight = committeeID
	proposal.Meta.DoubleSigners, err = cons.ProcessDSE(be.DSE.Evidence...)
	if err != nil {
		c.log.Warn(err.Error()) // still produce proposal
	}
	proposal.Meta.BadProposers, err = cons.ProcessBPE(be.BPE.Evidence...)
	if err != nil {
		c.log.Warn(err.Error()) // still produce proposal
	}
	return proposal, nil
}

func (c *Controller) BFTTriggerFinishedCallback(committeeID uint64) {
	c.Lock()
	defer c.Unlock()
	consensus, ok := c.Consensus[committeeID]
	if !ok {
		c.log.Errorf("failed retrieving bft object when trigger occurred %s", lib.ErrWrongCommitteeID())
		return
	}
	// TODO !!! handle async issues with canopy proper block conflicting with integrated chain
	var err lib.ErrorI
	consensus.LastValidatorSet = consensus.ValidatorSet
	consensus.ValidatorSet, err = c.FSM.GetCommittee(committeeID)
	if err != nil {
		c.log.Errorf("failed retrieving committee when trigger occurred %s", err.Error())
		return
	}
	consensus.ResetBFTChan() <- time.Duration(0)
}

// CANOPY SPECIFIC FUNCTIONALITY BELOW

func (c *Controller) ValidatorProposalConfig(fsm ...*fsm.StateMachine) (reset func()) {
	for _, f := range fsm {
		if c.Consensus[lib.CANOPY_COMMITTEE_ID].GetRound() < 3 {
			f.SetProposalVoteConfig(types.GovProposalVoteConfig_APPROVE_LIST)
		} else {
			f.SetProposalVoteConfig(types.GovProposalVoteConfig_REJECT_ALL)
		}
	}
	reset = func() {
		for _, f := range fsm {
			f.SetProposalVoteConfig(types.AcceptAllProposals)
		}
	}
	return
}

func (c *Controller) GetPendingPage(p lib.PageParams) (page *lib.Page, err lib.ErrorI) {
	c.Lock()
	defer c.Unlock()
	plug, _, err := c.GetPluginAndConsensus(lib.CANOPY_COMMITTEE_ID)
	if err != nil {
		return nil, err
	}
	return plug.(plugin.CanopyPlugin).PendingPageForRPC(p)
}
