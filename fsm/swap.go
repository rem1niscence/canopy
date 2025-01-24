package fsm

import (
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
)

// HandleCommitteeSwaps() when the committee submits a 'certificate results transaction', it informs the chain of various actions over sell orders
// - 'buy' is an actor 'claiming / reserving' the sell order
// - 'reset' is a 'claimed' order whose 'buyer' did not send the tokens to the seller before the deadline, thus the order is re-opened for sale
// - 'close' is a 'claimed' order whose 'buyer' sent the tokens to the seller before the deadline, thus the order is 'closed' and the tokens are moved from escrow to the buyer
func (s *StateMachine) HandleCommitteeSwaps(orders *lib.Orders, committeeId uint64) lib.ErrorI {
	if orders != nil {
		// buy orders are a result of the committee witnessing a 'reserve transaction' for the order on the 'buyer chain'
		// think of 'buy orders' like reserving the 'sell order'
		for _, buyOrder := range orders.BuyOrders {
			if err := s.BuyOrder(buyOrder, committeeId); err != nil {
				// buy orders may error when base-chain
				s.log.Warnf("BuyOrder failed (can happen due to race with base-chain): %s", err.Error())
			}
		}
		// reset orders are a result of the committee witnessing 'no-action' from the buyer of the sell order aka NOT sending the
		// corresponding assets before the 'deadline height' of the 'buyer chain'. The buyer address and deadline height are reset and the
		// sell order is listed as 'available' to the rest of the market
		for _, resetOrderId := range orders.ResetOrders {
			if err := s.ResetOrder(resetOrderId, committeeId); err != nil {
				return err
			}
		}
		// close orders are a result of the committee witnessing the buyer sending the
		// buy assets before the 'deadline height' of the 'buyer chain'
		for _, closeOrderId := range orders.CloseOrders {
			if err := s.CloseOrder(closeOrderId, committeeId); err != nil {
				return err
			}
		}
	}
	return nil
}

// ParseBuyOrder() parses a transaction for an embedded buy order messages in the memo field
func (s *StateMachine) ParseBuyOrder(tx *lib.Transaction, deadlineBlocks uint64) (bo *lib.BuyOrder, ok bool) {
	bo = new(lib.BuyOrder)
	if err := lib.UnmarshalJSON([]byte(tx.Memo), bo); err == nil {
		if len(bo.BuyerSendAddress) != 0 && len(bo.BuyerSendAddress) != 0 {
			ok = true
		}
		// set the buyer deadline
		bo.BuyerChainDeadline = s.Height() + deadlineBlocks
	}
	return
}

// ProcessBaseChainOrderBook() processes the order book from the base-chain and cross-references
func (s *StateMachine) ProcessBaseChainOrderBook(book *lib.OrderBook, b *lib.BlockResult) (closeOrders, resetOrders []uint64) {
	transferred := make(map[string]uint64) // [from+to] -> amount sent
	// get all the 'Send' transactions from the block
	for _, tx := range b.Transactions {
		// ignore non-send
		if tx.MessageType != types.MessageSendName {
			continue
		}
		// parse send
		msg, e := lib.FromAny(tx.Transaction.Msg)
		if e != nil {
			s.log.Error(e.Error())
			continue
		}
		send, ok := msg.(*types.MessageSend)
		if !ok {
			s.log.Error("Non-send message with a send message name")
			continue
		}
		// add to total transferred
		transferred[lib.BytesToString(append(tx.Sender, tx.Recipient...))] += send.Amount
	}
	// for each order
	for _, order := range book.Orders {
		// skip buyer-less orders
		if len(order.BuyerReceiveAddress) == 0 {
			continue
		}
		// extract a key for the totalTransferred map
		key := lib.BytesToString(append(order.BuyerSendAddress, order.SellerReceiveAddress...))
		// see if expired
		if s.height > order.BuyerChainDeadline {
			// add to reset orders
			resetOrders = append(resetOrders, order.Id)
		} else if transferred[key] == order.RequestedAmount {
			closeOrders = append(closeOrders, order.Id)
		}
	}
	return
}

// ParseBuyOrders() parses the proposal block for memo commands to execute specialized 'buy order' functionality
func (s *StateMachine) ParseBuyOrders(b *lib.BlockResult) (buyOrders []*lib.BuyOrder) {
	params, err := s.GetParams()
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	// calculate the minimum buy order fee
	minFee := params.Fee.MessageSendFee * params.Validator.ValidatorBuyOrderFeeMultiplier
	// for each transaction in the block
	for _, tx := range b.Transactions {
		deDupeBuyOrders := make(map[uint64]struct{})
		// skip over any that doesn't have the minimum fee or isn't the correct type
		if tx.MessageType != types.MessageSendName && tx.Transaction.Fee < minFee {
			continue
		}
		// parse the transaction for embedded 'buy orders'
		if buyOrder, ok := s.ParseBuyOrder(tx.Transaction, params.Validator.ValidatorBuyDeadlineBlocks); ok {
			if _, found := deDupeBuyOrders[buyOrder.OrderId]; !found {
				buyOrders = append(buyOrders, buyOrder)
				deDupeBuyOrders[buyOrder.OrderId] = struct{}{}
			}
		}
	}
	return
}

// CreateOrder() adds an order to the order book for a committee in the state db
func (s *StateMachine) CreateOrder(order *lib.SellOrder, committeeId uint64) (orderId uint64, err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	orderId = orderBook.AddOrder(order)
	err = s.SetOrderBook(orderBook)
	return
}

// EditOrder() updates an existing order in the order book for a committee in the state db
func (s *StateMachine) EditOrder(order *lib.SellOrder, committeeId uint64) (err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	if err = orderBook.UpdateOrder(int(order.Id), order); err != nil {
		return
	}
	err = s.SetOrderBook(orderBook)
	return
}

// BuyOrder() adds a recipient and a deadline height to an existing order and saves it to the state
func (s *StateMachine) BuyOrder(buyOrder *lib.BuyOrder, committeeId uint64) (err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	if err = orderBook.BuyOrder(int(buyOrder.OrderId), buyOrder.BuyerReceiveAddress, buyOrder.BuyerSendAddress, buyOrder.BuyerChainDeadline); err != nil {
		return
	}
	err = s.SetOrderBook(orderBook)
	return
}

// ResetOrder() removes the recipient and deadline height from an existing order and saves it to the state
func (s *StateMachine) ResetOrder(orderId, committeeId uint64) (err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	if err = orderBook.ResetOrder(int(orderId)); err != nil {
		return
	}
	err = s.SetOrderBook(orderBook)
	return
}

// CloseOrder() sends the tokens from escrow to the 'buyer address' and deletes the order
func (s *StateMachine) CloseOrder(orderId, committeeId uint64) (err lib.ErrorI) {
	// the order is 'closed' and the tokens are moved from escrow to the buyer
	order, err := s.GetOrder(orderId, committeeId)
	if err != nil {
		// due to the redundancy 'Look Back' design of the swaps submitting a close order that has no available order is allowed
		// this is considered safe due to the +2/3rd committee signature requirement
		s.log.Warn(err.Error())
		return nil
	}
	// ensure the order already was 'claimed / reserved'
	if order.BuyerReceiveAddress == nil {
		return types.ErrInvalidBuyOrder()
	}
	// remove the funds from the escrow pool
	if err = s.PoolSub(committeeId+types.EscrowPoolAddend, order.AmountForSale); err != nil {
		return err
	}
	// send the funds to the recipient address
	if err = s.AccountAdd(crypto.NewAddress(order.BuyerReceiveAddress), order.AmountForSale); err != nil {
		return err
	}
	// delete the order
	return s.DeleteOrder(orderId, committeeId)
}

// DeleteOrder() deletes an existing order in the order book for a committee in the state db
func (s *StateMachine) DeleteOrder(orderId, committeeId uint64) (err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	if err = orderBook.UpdateOrder(int(orderId), nil); err != nil {
		return
	}
	err = s.SetOrderBook(orderBook)
	return
}

// GetOrder() sets the order book for a committee in the state db
func (s *StateMachine) GetOrder(orderId uint64, committeeId uint64) (order *lib.SellOrder, err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return nil, err
	}
	return orderBook.GetOrder(int(orderId))
}

// SetOrderBook() sets the order book for a committee in the state db
func (s *StateMachine) SetOrderBook(b *lib.OrderBook) lib.ErrorI {
	// convert the order book into bytes
	orderBookBz, err := lib.Marshal(b)
	if err != nil {
		return err
	}
	// set the order book in the store
	return s.store.Set(types.KeyForOrderBook(b.CommitteeId), orderBookBz)
}

// SetOrderBooks() sets a series of OrderBooks in the state db
func (s *StateMachine) SetOrderBooks(b *lib.OrderBooks, supply *types.Supply) lib.ErrorI {
	for _, book := range b.OrderBooks {
		// convert the order book into bytes
		orderBookBz, err := lib.Marshal(book)
		if err != nil {
			return err
		}
		// write the order book for the committee to state
		key := types.KeyForOrderBook(book.CommitteeId)
		if err = s.store.Set(key, orderBookBz); err != nil {
			return err
		}
		// properly mint to the supply pool
		for _, order := range book.Orders {
			supply.Total += order.AmountForSale
			if err = s.PoolAdd(book.CommitteeId+uint64(types.EscrowPoolAddend), order.AmountForSale); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetOrderBook() retrieves the order book for a committee from the state db
func (s *StateMachine) GetOrderBook(committeeId uint64) (b *lib.OrderBook, err lib.ErrorI) {
	// initialize the order book variable
	b = new(lib.OrderBook)
	b.Orders, b.CommitteeId = make([]*lib.SellOrder, 0), committeeId
	// get order book bytes from the db
	bz, err := s.Get(types.KeyForOrderBook(committeeId))
	if err != nil {
		return
	}
	// convert order book bytes into the order book variable
	err = lib.Unmarshal(bz, b)
	return
}

// GetOrderBooks() retrieves all OrderBooks from the state db
func (s *StateMachine) GetOrderBooks() (b *lib.OrderBooks, err lib.ErrorI) {
	b = new(lib.OrderBooks)
	it, err := s.Iterator(types.OrderBookPrefix())
	if err != nil {
		return
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		id, e := types.IdFromKey(it.Key())
		if e != nil {
			return nil, e
		}
		book, e := s.GetOrderBook(id)
		if e != nil {
			return nil, e
		}
		b.OrderBooks = append(b.OrderBooks, book)
	}
	return
}
