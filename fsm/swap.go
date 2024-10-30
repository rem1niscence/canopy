package fsm

import (
	"github.com/ginchuco/canopy/fsm/types"
	"github.com/ginchuco/canopy/lib"
	"github.com/ginchuco/canopy/lib/crypto"
)

func (s *StateMachine) HandleCommitteeBuyOrders(orders *lib.Orders, committeeId uint64) lib.ErrorI {
	if orders != nil {
		// buy orders are a result of the committee witnessing a 'claim transaction' for the order on the 'buyer chain'
		// think of 'buy orders' like reserving the 'sell order'
		for _, buyOrder := range orders.BuyOrders {
			if err := s.BuyOrder(buyOrder.OrderId, buyOrder.BuyerReceiveAddress, buyOrder.BuyerChainDeadline, committeeId); err != nil {
				return err
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
			order, err := s.GetOrder(closeOrderId, committeeId)
			if err != nil {
				// due to the redundancy 'Look Back' design of the swaps submitting a close order that has no available order is allowed
				// this is considered safe due to the +2/3rd committee signature requirement
				s.log.Warn(err.Error())
				continue
			}
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
			if err = s.DeleteOrder(closeOrderId, committeeId); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateOrder() adds an order to the order book for a committee in the state db
func (s *StateMachine) CreateOrder(order *types.SellOrder, committeeId uint64) (orderId uint64, err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	orderId = orderBook.AddOrder(order)
	err = s.SetOrderBook(orderBook)
	return
}

// EditOrder() updates an existing order in the order book for a committee in the state db
func (s *StateMachine) EditOrder(order *types.SellOrder, committeeId uint64) (err lib.ErrorI) {
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
func (s *StateMachine) BuyOrder(orderId uint64, buyerAddress []byte, buyerChainDeadlineHeight, committeeId uint64) (err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return
	}
	if err = orderBook.BuyOrder(int(orderId), buyerAddress, buyerChainDeadlineHeight); err != nil {
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
func (s *StateMachine) GetOrder(orderId uint64, committeeId uint64) (order *types.SellOrder, err lib.ErrorI) {
	orderBook, err := s.GetOrderBook(committeeId)
	if err != nil {
		return nil, err
	}
	return orderBook.GetOrder(int(orderId))
}

// SetOrderBook() sets the order book for a committee in the state db
func (s *StateMachine) SetOrderBook(b *types.OrderBook) lib.ErrorI {
	// convert the order book into bytes
	orderBookBz, err := lib.Marshal(b)
	if err != nil {
		return err
	}
	// set the order book in the store
	return s.store.Set(types.KeyForOrderBook(b.CommitteeId), orderBookBz)
}

// SetOrderBooks() sets a series of OrderBooks in the state db
func (s *StateMachine) SetOrderBooks(b *types.OrderBooks, supply *types.Supply) lib.ErrorI {
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
func (s *StateMachine) GetOrderBook(committeeId uint64) (b *types.OrderBook, err lib.ErrorI) {
	// initialize the order book variable
	b = new(types.OrderBook)
	b.Orders, b.CommitteeId = make([]*types.SellOrder, 0), committeeId
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
func (s *StateMachine) GetOrderBooks() (b *types.OrderBooks, err lib.ErrorI) {
	b = new(types.OrderBooks)
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
