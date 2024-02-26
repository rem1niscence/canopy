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

