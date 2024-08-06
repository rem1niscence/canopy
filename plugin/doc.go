package plugin

// TODO
// - [x] Add committees at a state machine level
// - [ ] P2P internal validators listener should be sent the exact list of who to connect to (committees + consensus vals)
// - [ ] P2P limits are only imposed on entry and should not worry about still connected stale committee members
// - [ ] Controller should hold a `map[CommitteeID]->*BFT` objects - the creation of which is based on chains.json at startup
// - [ ] Transactions and Blocks should have their own dedicated channels based on the committee IDs - channel subscription is based on chains.json at startup
