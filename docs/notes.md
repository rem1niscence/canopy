### Unorganized notes that will eventually be the foundation for a specification

- Mempool prioritization of Transactions:
    - Required to handle full blocks
    - Higher fees get in blocks
    - Mempool will discard in FIFO manner regardless
    - Automated fee increases as reach top block size
    - Should prioritize governance level transactions
    - Block Limit is important to prevent early on protocol attack where blocks get filled very cheaply
    - Variable block size (nice to have) 20% above the current block size

- Pausing Txns
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

### Placeholder Name:

- **MMBFT**

### What is it?

- Merged Mining for **BFT** protocols

### What's the value add?

- **Incentivizing** validator participation in other BFT protocols by rewarding participants for Validating on 3rd party chains.
- **Bootstrapping** security for small chains
- **Sustaining** Validator participants during crisis events like Chain Halts or protocol downturn

### Summary

MMBFT **Listeners** come to consensus on the `ValidatorSet` of external chains and **Members** of the `ValidatorSet` registered on MMBFT are rewarded with MMBFT token.

MMBFT **Listeners** set are split into `Committees` based on which external chain they are `listening` on and their VotingPower within the committee is proportional to their **StakeAmount**. These `Committees` vote on how the block reward is distributed for that chain. If **Listeners** cannot come to consensus, no reward is distributed for the chain.

The block reward is static and equal for all supported external chains and is distributed to both **Members** proportional to `VotingPowerChainX` vs the `TotalVotingPowerChainX` that is registered on the MMBFT protocol and **Listeners'** proportional to `VotingPowerOnMMBFT` vs `TotalCommitteeVotingPower`.

MMBFT **Members'** compensation increases dramatically when they are staked on the MMBFT protocol and **Members** may delegate the staking responsibility to an **Operator** within the **RegisterTxn**.


