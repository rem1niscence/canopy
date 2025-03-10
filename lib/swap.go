package lib

import (
	"encoding/json"
)

/* This file implements 'sell order book' logic for token swaps that is used throughout the app */

// AddOrder() adds a sell order to the OrderBook
func (x *OrderBook) AddOrder(order *SellOrder) (id uint64) {
	// if there's an empty slot, fill it with the sell order
	for i, slot := range x.Orders {
		// if a slot is empty
		if slot.Empty() {
			// set the order id to index
			id = uint64(i)
			// set the structure order id to 'id'
			order.Id = id
			// set the 'order' in the book
			x.Orders[i] = order
			// exit
			return
		}
	}
	// if there's no empty slots, add the sell order to the end
	id = uint64(len(x.Orders))
	// set the structure order id to 'id'
	order.Id = id
	// add to the end of the book
	x.Orders = append(x.Orders, order)
	// exit
	return
}

// LockOrder() adds a 'buyer' recipient and send address and deadline height to the order to 'reserve' the order and prevent others from 'reserving it'
func (x *OrderBook) LockOrder(orderId int, buyersReceiveAddress, buyersSendAddress []byte, buyerChainDeadlineHeight uint64) (err ErrorI) {
	// get the order from the order book using the 'order id'
	order, err := x.GetOrder(orderId)
	// if an error occurred during retrieval
	if err != nil {
		// exit with error
		return
	}
	// if the buyer's receive address isn't nil
	if order.BuyerReceiveAddress != nil {
		// exit with 'order already locked' error
		return ErrOrderLocked()
	}
	// set the buyer's receive, send, and deadline height in the order
	order.BuyerReceiveAddress, order.BuyerSendAddress, order.BuyerChainDeadline = buyersReceiveAddress, buyersSendAddress, buyerChainDeadlineHeight
	// update the order in the order book
	x.Orders[orderId] = order
	// exit
	return
}

// ResetOrder() removes a 'lock' from the order to 'un-reserve' the order
func (x *OrderBook) ResetOrder(orderId int) (err ErrorI) {
	// get the order from the order book using the 'order id'
	order, err := x.GetOrder(orderId)
	// if an error occurred during retrieval
	if err != nil {
		// exit with error
		return
	}
	// reset the buyer's receive, send, and deadline height in the order
	order.BuyerReceiveAddress, order.BuyerSendAddress, order.BuyerChainDeadline = nil, nil, 0
	// update the order in the order book
	x.Orders[orderId] = order
	// exit
	return
}

// UpdateOrder() updates a sell order to the OrderBook, passing a nil `order` is effectively a delete operation
func (x *OrderBook) UpdateOrder(orderId int, order *SellOrder) (err ErrorI) {
	// create a variable to track the 'max index' of order slots
	maxIdx := len(x.Orders) - 1
	// if the order id exceeds the max index
	if orderId > maxIdx {
		// exit with 'not found' error
		return ErrOrderNotFound(orderId)
	}
	// if deleting from the end, shrink the slice
	if order == nil && orderId == maxIdx {
		// remove the final order from the slice
		x.Orders = x.Orders[:maxIdx]
		// continue shrinking the slice if nil entries are at the end
		for i := maxIdx - 1; i >= 0; i-- {
			// if the order slot is not empty
			if !x.Orders[i].Empty() {
				// exit the loop
				break
			}
			// shrink the slice by 1
			x.Orders = x.Orders[:i]
		}
		// exit
		return
	}
	// if not deleting from the end of the slice, simply replace the order
	x.Orders[orderId] = order
	// exit
	return
}

// GetOrder() retrieves a sell order from the OrderBook
func (x *OrderBook) GetOrder(orderId int) (order *SellOrder, err ErrorI) {
	// create a variable to track the 'max index' of order slots
	maxIdx := len(x.Orders) - 1
	// if the order id exceeds the max index
	if orderId > maxIdx || x.Orders[orderId].Empty() {
		// exit with 'not found' error
		return nil, ErrOrderNotFound(orderId)
	}
	// return the order at the index
	order = x.Orders[orderId]
	// exit
	return
}

// Empty() indicates whether the sell order is null
func (x *SellOrder) Empty() bool {
	return x == nil || x.SellersSendAddress == nil
}

// jsonSellOrder is the json.Marshaller and json.Unmarshaler implementation for the SellOrder object
type jsonSellOrder struct {
	Id                   uint64   `json:"id,omitempty"`                   // the unique identifier of the order
	Committee            uint64   `json:"committee,omitempty"`            // the id of the committee that is in-charge of escrow for the swap
	AmountForSale        uint64   `json:"amountForSale,omitempty"`        // amount of CNPY for sale
	RequestedAmount      uint64   `json:"requestedAmount,omitempty"`      // amount of 'token' to receive
	SellerReceiveAddress HexBytes `json:"sellerReceiveAddress,omitempty"` // the external chain address to receive the 'token'
	BuyerSendAddress     HexBytes `json:"buyerSendAddress,omitempty"`     // the send address from the buyer
	BuyerReceiveAddress  HexBytes `json:"buyerReceiveAddress,omitempty"`  // the buyers address to receive the 'coin'
	BuyerChainDeadline   uint64   `json:"buyerChainDeadline,omitempty"`   // the external chain height deadline to send the 'tokens' to SellerReceiveAddress
	SellersSellAddress   HexBytes `json:"sellersSendAddress,omitempty"`   // the address of seller who is selling the 'coin'
}

// MarshalJSON() is the json.Marshaller implementation for the SellOrder object
func (x SellOrder) MarshalJSON() ([]byte, error) {
	return json.Marshal(jsonSellOrder{
		Id:                   x.Id,
		Committee:            x.Committee,
		AmountForSale:        x.AmountForSale,
		RequestedAmount:      x.RequestedAmount,
		SellerReceiveAddress: x.SellerReceiveAddress,
		BuyerSendAddress:     x.BuyerSendAddress,
		BuyerReceiveAddress:  x.BuyerReceiveAddress,
		BuyerChainDeadline:   x.BuyerChainDeadline,
		SellersSellAddress:   x.SellersSendAddress,
	})
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the SellOrder object
func (x *SellOrder) UnmarshalJSON(jsonBytes []byte) (err error) {
	// create a new json object reference to ensure a non nil result
	j := new(jsonSellOrder)
	// populate the json object using the json bytes
	if err = json.Unmarshal(jsonBytes, j); err != nil {
		// exit with error
		return
	}
	// populate the underlying sell order using the json object
	*x = SellOrder{
		Id:                   j.Id,
		Committee:            j.Committee,
		AmountForSale:        j.AmountForSale,
		RequestedAmount:      j.RequestedAmount,
		SellerReceiveAddress: j.SellerReceiveAddress,
		BuyerSendAddress:     j.BuyerSendAddress,
		BuyerReceiveAddress:  j.BuyerReceiveAddress,
		BuyerChainDeadline:   j.BuyerChainDeadline,
		SellersSendAddress:   j.SellersSellAddress,
	}
	// exit
	return
}

// MarshalJSON() is the json.Marshaller implementation for the OrderBooks object
func (x OrderBooks) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.OrderBooks)
}

// UnmarshalJSON() is the json.Unmarshaler implementation for the OrderBooks object
func (x *OrderBooks) UnmarshalJSON(jsonBytes []byte) (err error) {
	// create a new json object ref to ensure a non nil result
	jsonOrderBooks := new([]*OrderBook)
	// populate the object using json bytes
	if err = json.Unmarshal(jsonBytes, jsonOrderBooks); err != nil {
		// exit
		return
	}
	// populate the underlying object using the json object
	*x = OrderBooks{
		OrderBooks: *jsonOrderBooks,
	}
	// exit
	return
}
