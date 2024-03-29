package leader_election

/*
	Use CDF + practical VRF which is Hash(BLS.Signature(Last 5 Proposer PubKey + Height + View))
	Fallback to Round Robin if none found

	- protects against grinding attack
	- protects against proposer ddos
	- weights based on stake

*/
