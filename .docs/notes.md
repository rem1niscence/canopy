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

- FAULTY LEADER
    - Election Vote QC (Propose Msg) for round-1 that has `+2/3Maj` but where `leaderKey` != `producerKey`
    - Evidence is tracked via `HeightLeaderMessages` as `ProposeMessages`
    - Evidence is submitted by replicas to leader during the Election Vote phase of the next round
    - Evidence is summarized in the next block by the leader and included in the QC to convince replicas

- DOUBLE SIGNER
    - A Vote QC (Propose, Precommit or Commit Msg) for round-1 where a different QC exists for the same view `(h,r,p)`
    - Evidence is tracked via `HeightLeaderMessages` as `LeaderMessages` and `HeightVoteSet` as `ElectionVoteQC`
    - Evidence is submitted by replicas to leader during the Election Vote phase of the next round
    - Evidence is summarized in the next block by the leader and included in the QC to convince replicas

- NON SIGNER // TODO Leader block reward should also be degraded based on NON SIGNERS to prevent against omission
  attacks
    - Precommit QC (Commit Msg) for the previous height that has `+2/3Maj` but isn't signed by a Validator
    - Evidence is tracked via Leader `SignatureAggregation`
    - Evidence is included in the next block by the next leader

NOTE: Round-1 includes the latest round of height-1 if round == 0

Long Range Attack
- Attack: An attacker bribes +2/3 voting power of an unstaked set of Validators to get their now worthless keys. he forks the chain 
and convinces new joiners to come on his fork.
- Solution: Given a trusted block A and a Validator-Set V-A and the next trusted block B and the Validator-Set V-B:
  +2/3 of V-A stake is in V-B. Since at point A there were at least +2/3rd honest validator power in the system, it's 
  impossible to fork the chain given that +1/3 of V-B stake must be honest.

Short Range Attack:
- Attack: Validators are incentivized to vote on the cannonical chain. It doesn't cost them anything to vote on two competing forks
  This means they are incentivized to double sign which would break the BFT safety.
- Solution: Given a trusted block A and a Validator-Set V-A and the next trusted block B and the Validator-Set V-B:
  +2/3 of V-A stake is in V-B. Slash any evidence of double signing from A-B. This mechanism disincentives double signing between
  trusted blocks. If slashing cannot occur due to a loss of +2/3 majority on the cannonical chain, the chain will halt, preserving safety,
  until social recovery may occur. 

Mechanisms Needed:
- Trusted Blocks (Cementing/Checkpointing), every UNSTAKING_BLOCKS, checkpoint the software 

- FAQ:
    - Q: Since Evidence must be from the view-1, isn't it possible evidence may go unreported in Type 2 async networks?
    - A: The consensus mechanism waits a max-network-delay delta time bound to prevent the hidden lock issue. This is
      the entire basis
      of the `lock + highQC` design and is considered a peer-reviewed safeguard of the liveness of the protocol. Since
      Canopy adopts the same
      mechanism for evidence, the security guarantee is the same. Thus Canopy does not need to look any further back
      than height-1 for
      evidence.
    - Correction: it's not about the evidence going unreported, it's about the evidence happening after the reporting
      period is over and providing
      no mechanism to punish double signers. This means short-range attacks may happen at any block < n-1. This is not
      an acceptable security measure.

    - Q: Why aren't Replica Votes structures used as a type of Double Sign evidence.
    - A: Replica Double Votes are always summarized in the form of `2 QCs` as it's more scalable to track multiple
      double signers.
      It'c not a security issue because if the reporter is a Leader Candidate than they have the ability to make a QC
      and will receive
      an equivocating QC from the Leader and if the reporter is a Replica, then they received the equivocating QCs from
      two leaders.

Brain Dump: Theoretically, a double signed Replica Message vote is a malicious thing - however without it being tied to
a Leader Message,
the vote is useless and expensive to track. Let's not forget that in the linear message complexity design, all messages
must run through
a `Leader` of sorts. Let's say a secondary byzantine leader is trying to get signatures for a conflicting view and
block, they'd get
a vote from the `double-signer` but that wouldn't be enough to do anything. The Byzantine Leader would need +2/3Maj
signatures to fork
the chain. Since the upper bound of faulty (double signers) validators if +1/3, he must ask 'good validators' to double
sign. Thus,
`double-signers` *must be outed* to pull of an attack to 'good validators'. Another scenario is if a Replica Double
Signs Election Vote
Messages, and a good Leader Candidate can prove that they signed for them and the true leader by reporting
the `partial Election VoteQC`
and the`+2/3Maj Election Vote QC`. Since `Replicas` only receive messages from Leaders, the double-sign-evidence must
come from Leader Messages
in the form of an `partial QC`.

- Height 0 should be the database version of the genesis begin state and height 1 should be the database version of the
  genesis end state. There are no quorum certificates associated with height 0 and height 1

### Placeholder Name:

- **Canopy**

### What is it?

- Merged Mining for **BFT** protocols

### What'c the value add?

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