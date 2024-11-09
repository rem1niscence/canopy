### Hotstuff 2 Phase with VRF+CDF Leader Election
ELECTION
- Replicas run the VRF and if a candidate they send VRF Out to the replicas

ELECTION-VOTE
- Replicas send ELECTION votes on the Leader: lowest VRF - if none fallback to round-robin

PROPOSE
- Leader receives +2/3 signatures ELECTION messages. This message also contains lock of each replica
- If     highLock.Height == curHeight then leader uses highLock.block as proposalBlock
- else   leader 'extends' the block (creates next block after highLock.block)
- Leader sends HighQC and the new proposal block to the replicas

PROPOSE-VOTE
- Replicas check the validity of QC.Votes (ELECTION votes)
- Replicas check the validity of the block using SafeNodePredicate:
```go
    if lockQC != nil {
        - Liveness
            - Unlock if proposal.QC.View > locked.QC.View AND proposal.BlockHash == QC.BlockHash
        - Safety
            - Unlock if proposal.BlockHash == locked.BlockHash
    }
  ```
- Send a signed vote back to the Leader

PRECOMMIT
- Leader receives +2/3 signatures from replicas confirming the validity of PROPOSE-MSG and sends the votes to the replicas

PRECOMMIT-VOTE
- Replicas check the validity of QC.Votes (PROPOSE votes), set `highQC` to QC
- Replicas send a signed vote back to the Leader

COMMIT
- Leader receives +2/3 signatures from replicas confirming the validity of the PRECOMMIT-MSG and sends the votes to the replicas
- Replicas check the validity of QC.Votes, commit the block to storage, increment the height, reset the round, unlock and start over

ROUND_INTERRUPT
- If delta time expires at any phase
- Increment the round number
- Start over at LEADER phase
- Timeouts increase quadratically with each round

PACEMAKER
- Jumps to the highest round where 2/3+ of the validators are on or have seen (via replica gossip O(n^2) communication complexity)
- If 1/3 Validators are on round 5 and 1/3 are on round 3 - pacemaker will set to round 3

ATTACKS:
- Leader omits votes to attack replicas?
- Higher round leader hijacking consensus?

NOTES:
- Each phase waits delta time bound for consistent block times and to prevent hidden lock problem

- A COMMIT message is used as justification to add a block to the blockchain for any node, however consensus on the
  commit message isn't able to be achieved until height+1 since there'c not an additional phase guaranteeing the
  exact contents of the commit message. Meaning a commit message is enough proof that we can commit the current block
  but not enough proof to add it to the block, modifying the block hash, and make state changes based on it until the
  next height. Even if an additional phase of consensus was added, the block may become invalid based on the COMMIT message.

  Thus, while syncing the blocks are verified the same height, but the contents of the commit message aren't applied until
  height+1 e.g rewarding the proposer, slashing non-signers etc.

FAQ:
- Why not 3 phase hotstuff for optimistic responsiveness?
	- 3rd phase solves the 'hidden lock' problem by running a precursor phase to ensure there'c enough evidence among the replicas that if
	  a node locked on a block, 2/3 replicas have also seen that value.
	- However, it'c very unlikely that a hidden lock would accidentally happen, this additional phase is only helpful in
	  a MessageType 2 asynchronous network where the hidden lock is forced by a malicious leader and the next leader is not malicious as the use of highQC
	  is never enforced among the replicas. It'c only enforced if they have locked on it. So they solve the accidental/single malicious hidden lock.
	- Also optimistic responsiveness is counter to blockchains that want relatively consistent block times
	- Not to mention, VRF leader selection requires some delta time bound to ensure no 'hidden leaders' and if we embed the VRF process within 3 phase
	  hotstuff it ends up becoming attackable by the current leader via omission or can forgo linear communication complexity.