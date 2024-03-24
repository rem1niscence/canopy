### Unorganized notes that will eventually be the foundation for a specification

- Mempool prioritization of Transactions:
    - Required to handle full blocks
    - Higher fees get in blocks
    - Mempool will discard in FIFO manner regardless
    - Automated fee increases as reach top block size
    - Should prioritize governance level transactions
    - Block Limit is important to prevent early on protocol attack where blocks get filled very cheaply
    - Variable block size (nice to have) 20% above the current block size

- Pausing Txs
    - Manual Pause: Is necessary to allow Validator to do maintenance
    - Automatic Pause: Is necessary to allow Consensus to recover from a faulty / malicious Validator

- Slashing
    - Used to penalize malicious behavior
    - Keep it very low for a long time because it usually is due to a protocol error (from our experience)

- Should include token bridge out of the box to ensure early liquidity viability

- 2 Store model
    - https://docs.cosmos.network/v0.46/architecture/adr-040-storage-and-smt-state-commitments.html

- Sparse Merkle Tree
    - https://lisk.com/blog/posts/sparse-merkle-trees-and-new-state-model

- Validators should be able to have the ability to lock parameters or unlock parameter changes

State Machine Handling Byzantine Evidence:

- Bad proposers, double signers, non signers and minority signers are proven via the finalized quorum certificate from
  the majority
- Bad proposers are easily retraced by seeing who were the proposers for all the rounds before the finalized round
- Non signers are who didn's sign on the final QC
- Minority signers are who signed faulty on the final QC
- Double signers tracking is less straightforward, but likely can be included in the QC somehow

### Placeholder Name:

- **Canopy**

### What is it?

- Merged Mining for **BFT** protocols

### What's the value add?

- **Incentivizing** validator participation in other BFT protocols by rewarding participants for Validating on 3rd party
  chains.
- **Bootstrapping** security for small chains
- **Sustaining** Validator participants during crisis events like Chain Halts or protocol downturn

### Summary

CanopyValidator

- Validates the canopy chain and only paid through block reward

CommitteeMembers <Must be CanopyValidators)

- Reports the validator-set of the client chain and come to a consensus on the state of that chain with other committee
  members
- Multi-Committe-Membership is accepted but requires additional stake

ClientValidator

- No minimum stake, but payment is degraded based on bucket

Features:

- Auto-Compounding stake feature is needed
- First transaction is free if you're a validator on a client chain
- Donate transactions: donate to client chain reward pool with a drain height
- 2 tiered Whitelisting of client chains based on Validator power + validator vote
- Key algorithm agnostic protocol

Attacks:
To be listed

Open question:

- Staking process especially with multi-chain