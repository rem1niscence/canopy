package controller

import (
	"bytes"
	"github.com/canopy-network/canopy/bft"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"slices"
)

func (c *Controller) NewCertificateResults(block *lib.Block, blockResult *lib.BlockResult, evidence *bft.ByzantineEvidence) (results *lib.CertificateResult) {
	// calculate reward recipients, creating a 'certificate results' object reference in the process
	results = c.CalculateRewardRecipients(block.BlockHeader.ProposerAddress, c.RootChainHeight())
	// handle swaps
	c.HandleSwaps(blockResult, results, c.RootChainHeight())
	// set slash recipients
	c.CalculateSlashRecipients(results, evidence)
	// set checkpoint
	c.CalculateCheckpoint(blockResult, results)
	// handle retired status
	c.HandleRetired(results)
	// exit
	return
}

// SendCertificateResultsTx() originates and auto-sends a CertificateResultsTx after successfully leading a Consensus height
func (c *Controller) SendCertificateResultsTx(qc *lib.QuorumCertificate) {
	// get the root chain id from the state
	rootChainId, err := c.FSM.GetRootChainId()
	if c.Config.ChainId == rootChainId {
		return // root-Chain doesn't send this
	}
	c.log.Debugf("Sending certificate results txn for: %s", lib.BytesToString(qc.ResultsHash))
	// save the block to set it back to the object after this function completes
	blk := qc.Block
	defer func() { qc.Block = blk }()
	// it's good practice to omit the block when sending the transaction as it's not relevant to canopy
	qc.Block = nil
	tx, err := types.NewCertificateResultsTx(c.PrivateKey, qc, rootChainId, c.Config.NetworkID, 0, c.RootChainHeight(), "")
	if err != nil {
		c.log.Errorf("Creating auto-certificate-results-txn failed with err: %s", err.Error())
		return
	}
	// handle the transaction on the root-Chain
	hash, err := c.RootChainInfo.RemoteCallbacks.Transaction(tx)
	if err != nil {
		c.log.Errorf("Submitting auto-certificate-results-txn failed with err: %s", err.Error())
		return
	}
	c.log.Infof("Successfully submitted the certificate-results-txn with hash %s", *hash)
}

// CalculateRewardRecipients() calculates the block reward recipients of the proposal
func (c *Controller) CalculateRewardRecipients(proposerAddress []byte, rootChainHeight uint64) (results *lib.CertificateResult) {
	// get the root chain id
	rootChainId, err := c.FSM.GetRootChainId()
	if err != nil {
		c.log.Warnf("An error occurred getting the root chain id from state: %s", err.Error())
	}
	// if own root
	isOwnRoot := rootChainId == c.Config.ChainId
	// set block reward recipients
	results = &lib.CertificateResult{
		RewardRecipients: &lib.RewardRecipients{},
		SlashRecipients:  new(lib.SlashRecipients),
	}
	// create variables to hold potential 'reward recipients'
	var delegate, nValidator, nDelegate *lib.LotteryWinner
	// start the proposer with a 100% allocation
	proposer := &lib.LotteryWinner{Winner: proposerAddress, Cut: 100}
	// get the delegate and their cut from the state machine
	delegate, err = c.GetRootChainLotteryWinner(rootChainHeight)
	if err != nil {
		c.log.Warnf("An error occurred choosing a root chain delegate lottery winner: %s", err.Error())
		// continue
	}
	// add the delegate as a reward recipient, subtracting share away from the proposer
	c.AddRewardRecipient(proposer, delegate, results, isOwnRoot, rootChainId)
	// if this chain isn't root chain
	if !isOwnRoot {
		// get the nested-validator lottery winner for the 'self chain' (if self is not root)
		nValidator, err = c.FSM.LotteryWinner(c.Config.ChainId, true)
		if err != nil {
			c.log.Warnf("An error occurred choosing a nested-validator lottery winner: %s", err.Error())
			// continue
		}
		// add the nested validator as a reward recipient, subtracting share away from the proposer
		c.AddRewardRecipient(proposer, nValidator, results, isOwnRoot, rootChainId)
		// get the nested-delegate lottery winner for the 'self chain' (if self is not root)
		nDelegate, err = c.FSM.LotteryWinner(c.Config.ChainId)
		if err != nil {
			c.log.Warnf("An error occurred choosing a nested-delegate lottery winner: %s", err.Error())
			// continue
		}
		// add the nested delegate as a reward recipient, subtracting share away from the proposer
		c.AddRewardRecipient(proposer, nDelegate, results, isOwnRoot, rootChainId)
	}
	// finally add the proposer at the end after ensuring their proper percent
	c.AddPaymentPercent(proposer, results, isOwnRoot, rootChainId)
	return
}

// AddRewardRecipient() adds a reward recipient to the list of reward recipients in the certificate result
func (c *Controller) AddRewardRecipient(proposer, toAdd *lib.LotteryWinner, results *lib.CertificateResult, isOwnRoot bool, rootChainId uint64) {
	// skip any nil recipient
	if toAdd == nil || len(toAdd.Winner) == 0 {
		// exit
		return
	}
	// ensure the new recipient's cut doesn't underflow the proposers cut
	if proposer.Cut < toAdd.Cut {
		c.log.Warnf("Not enough proposer cut for winner")
		// exit
		return
	}
	// calculate new proposer cut
	proposer.Cut -= toAdd.Cut
	// add the payment percent for the recipient
	c.AddPaymentPercent(toAdd, results, isOwnRoot, rootChainId)
}

// AddPaymentPercent() adds a payment percent to the certificate result
func (c *Controller) AddPaymentPercent(toAdd *lib.LotteryWinner, results *lib.CertificateResult, isOwnRoot bool, rootChainId uint64) {
	c.addPaymentPercent(toAdd, results, rootChainId)
	// if this chain is not its own root (nested chain)
	if !isOwnRoot {
		c.addPaymentPercent(toAdd, results, c.Config.ChainId)
	}
}

// addPaymentPercent() is a helper function to add a payment percent to the certificate result
func (c *Controller) addPaymentPercent(toAdd *lib.LotteryWinner, results *lib.CertificateResult, chainId uint64) {
	// check if the winner's address is already in the reward recipients list for the root chain
	// if found, update their reward percentage - else add them as a new recipient
	if !slices.ContainsFunc(results.RewardRecipients.PaymentPercents, func(pp *lib.PaymentPercents) (has bool) {
		// if the address matches and belongs to the root chain
		if bytes.Equal(pp.Address, toAdd.Winner) && pp.ChainId == chainId {
			// mark as found
			has = true
			// increase their reward percentage by 'cut'
			pp.Percent += toAdd.Cut
		}
		return
	}) {
		// if the winner is not found in the list, add them as a new recipient
		results.RewardRecipients.PaymentPercents = append(results.RewardRecipients.PaymentPercents,
			&lib.PaymentPercents{
				Address: toAdd.Winner,
				Percent: toAdd.Cut,
				ChainId: chainId,
			})
	}
}

// HandleSwaps() handles the 'buy' side of the sell orders
func (c *Controller) HandleSwaps(blockResult *lib.BlockResult, results *lib.CertificateResult, rootChainHeight uint64) {
	// parse the last block for buy orders and polling
	buyOrders := c.FSM.ParseBuyOrders(blockResult)
	// get orders from the root-Chain
	orders, e := c.LoadRootChainOrderBook(rootChainHeight)
	if e != nil {
		return
	}
	// process the root-Chain order book against the state
	closeOrders, resetOrders := c.FSM.ProcessRootChainOrderBook(orders, blockResult)
	// add the orders to the certificate result
	// truncate for defensive spam protection
	results.Orders = &lib.Orders{
		BuyOrders:   lib.TruncateSlice(buyOrders, 1000),
		ResetOrders: resetOrders,
		CloseOrders: closeOrders,
	}
}

// CalculateSlashRecipients() calculates the addresses who receive slashes on the root-Chain
func (c *Controller) CalculateSlashRecipients(results *lib.CertificateResult, be *bft.ByzantineEvidence) {
	var err lib.ErrorI
	// use the bft object to fill in the Byzantine Evidence
	results.SlashRecipients.DoubleSigners, err = c.Consensus.ProcessDSE(be.DSE.Evidence...)
	if err != nil {
		c.log.Warn(err.Error()) // still produce proposal
	}
	if srLen := len(results.SlashRecipients.DoubleSigners); srLen != 0 {
		c.log.Infof("Added %d slash recipients due to byzantine evidence", srLen)
	}
}

const CheckpointFrequency = 100

// CalculateCheckpoint() calculates the checkpoint for the checkpoint as a service functionality
func (c *Controller) CalculateCheckpoint(blockResult *lib.BlockResult, results *lib.CertificateResult) {
	// checkpoint every 100 heights
	if blockResult.BlockHeader.Height%CheckpointFrequency == 0 {
		c.log.Info("Checkpoint set in certificate results")
		results.Checkpoint = &lib.Checkpoint{
			Height:    blockResult.BlockHeader.Height,
			BlockHash: blockResult.BlockHeader.Hash,
		}
	}
}

// HandleRetired() checks if the committee is retiring and sets in the results accordingly
func (c *Controller) HandleRetired(results *lib.CertificateResult) {
	cons, err := c.FSM.GetParamsCons()
	if err != nil {
		c.log.Error(err.Error())
		return
	}
	results.Retired = cons.Retired != 0
}
