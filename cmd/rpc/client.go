package rpc

import (
	"bytes"
	"fmt"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"io"
	"net/http"
)

type Client struct {
	rpcURL  string
	rpcPort string
	client  http.Client
}

func NewClient(rpcURL, rpcPort string) *Client {
	return &Client{rpcURL: rpcURL, rpcPort: rpcPort, client: http.Client{}}
}

func (c *Client) Version() (version *string, err lib.ErrorI) {
	version = new(string)
	err = c.get(VersionRouteName, version)
	return
}

func (c *Client) Height() (p *uint64, err lib.ErrorI) {
	p = new(uint64)
	err = c.post(HeightRouteName, nil, p)
	return
}

func (c *Client) BlockByHeight(height uint64) (p *lib.BlockResult, err lib.ErrorI) {
	p = new(lib.BlockResult)
	err = c.heightRequest(BlockByHeightRouteName, height, p)
	return
}

func (c *Client) BlockByHash(hash string) (p *lib.BlockResult, err lib.ErrorI) {
	p = new(lib.BlockResult)
	err = c.hashRequest(BlockByHashRouteName, hash, p)
	return
}

func (c *Client) Blocks(params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(BlocksRouteName, 0, params, p)
	return
}

func (c *Client) Pending(params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedAddrRequest(PendingRouteName, "", params, p)
	return
}

func (c *Client) CertByHeight(height uint64) (p *lib.QuorumCertificate, err lib.ErrorI) {
	p = new(lib.QuorumCertificate)
	err = c.heightRequest(CertByHeightRouteName, height, p)
	return
}

func (c *Client) TransactionByHash(hash string) (p *lib.TxResult, err lib.ErrorI) {
	p = new(lib.TxResult)
	err = c.hashRequest(TxByHashRouteName, hash, p)
	return
}

func (c *Client) TransactionsByHeight(height uint64, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(TxsByHeightRouteName, height, params, p)
	return
}

func (c *Client) TransactionsBySender(address string, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedAddrRequest(TxsBySenderRouteName, address, params, p)
	return
}

func (c *Client) TransactionsByRecipient(address string, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedAddrRequest(TxsByRecRouteName, address, params, p)
	return
}

func (c *Client) Account(height uint64, address string) (p *types.Account, err lib.ErrorI) {
	p = new(types.Account)
	err = c.heightAndAddressRequest(AccountRouteName, height, address, p)
	return
}

func (c *Client) Accounts(height uint64, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(AccountsRouteName, height, params, p)
	return
}

func (c *Client) Pool(height uint64, name string) (p *types.Pool, err lib.ErrorI) {
	p = new(types.Pool)
	err = c.heightAndNameRequest(PoolRouteName, height, name, p)
	return
}

func (c *Client) Pools(height uint64, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(PoolsRouteName, height, params, p)
	return
}

func (c *Client) Validator(height uint64, address string) (p *types.Validator, err lib.ErrorI) {
	p = new(types.Validator)
	err = c.heightAndAddressRequest(ValidatorRouteName, height, address, p)
	return
}

func (c *Client) Validators(height uint64, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(ValidatorsRouteName, height, params, p)
	return
}

func (c *Client) ConsValidators(height uint64, params lib.PageParams) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(ConsValidatorsRouteName, height, params, p)
	return
}

func (c *Client) Supply(height uint64) (p *types.Supply, err lib.ErrorI) {
	p = new(types.Supply)
	err = c.heightRequest(SupplyRouteName, height, p)
	return
}

func (c *Client) NonSigners(height uint64) (p *types.NonSigners, err lib.ErrorI) {
	p = new(types.NonSigners)
	err = c.heightRequest(NonSignersRouteName, height, p)
	return
}

func (c *Client) Params(height uint64) (p *types.Params, err lib.ErrorI) {
	p = new(types.Params)
	err = c.heightRequest(ParamRouteName, height, p)
	return
}

func (c *Client) FeeParams(height uint64) (p *types.FeeParams, err lib.ErrorI) {
	p = new(types.FeeParams)
	err = c.heightRequest(FeeParamRouteName, height, p)
	return
}

func (c *Client) GovParams(height uint64) (p *types.GovernanceParams, err lib.ErrorI) {
	p = new(types.GovernanceParams)
	err = c.heightRequest(GovParamRouteName, height, p)
	return
}

func (c *Client) ConParams(height uint64) (p *types.ConsensusParams, err lib.ErrorI) {
	p = new(types.ConsensusParams)
	err = c.heightRequest(ConParamsRouteName, height, p)
	return
}

func (c *Client) ValParams(height uint64) (p *types.ValidatorParams, err lib.ErrorI) {
	p = new(types.ValidatorParams)
	err = c.heightRequest(ValParamRouteName, height, p)
	return
}

func (c *Client) State(height uint64) (p *types.GenesisState, err lib.ErrorI) {
	p = new(types.GenesisState)
	err = c.heightRequest(StateRouteName, height, p)
	return
}

func (c *Client) paginatedHeightRequest(routeName string, height uint64, p lib.PageParams, ptr any) (err lib.ErrorI) {
	bz, err := lib.JSONMarshal(paginatedHeightRequest{
		heightRequest: heightRequest{height},
		PageParams:    p,
	})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) paginatedAddrRequest(routeName string, address string, p lib.PageParams, ptr any) (err lib.ErrorI) {
	addr, err := lib.StringToBytes(address)
	if err != nil {
		return err
	}
	bz, err := lib.JSONMarshal(paginatedAddressRequest{
		addressRequest: addressRequest{addr},
		PageParams:     p,
	})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) heightRequest(routeName string, height uint64, ptr any) (err lib.ErrorI) {
	bz, err := lib.JSONMarshal(heightRequest{Height: height})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) hashRequest(routeName string, hash string, ptr any) (err lib.ErrorI) {
	bz, err := lib.JSONMarshal(hashRequest{Hash: hash})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) heightAndAddressRequest(routeName string, height uint64, address string, ptr any) (err lib.ErrorI) {
	addr, err := lib.StringToBytes(address)
	if err != nil {
		return err
	}
	bz, err := lib.JSONMarshal(heightAndAddressRequest{
		heightRequest:  heightRequest{height},
		addressRequest: addressRequest{addr},
	})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) heightAndNameRequest(routeName string, height uint64, name string, ptr any) (err lib.ErrorI) {
	bz, err := lib.JSONMarshal(heightAndNameRequest{
		heightRequest: heightRequest{height},
		nameRequest:   nameRequest{name},
	})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) Transaction(tx lib.TransactionI) (hash *string, err lib.ErrorI) {
	bz, err := lib.JSONMarshal(tx)
	if err != nil {
		return nil, err
	}
	hash = new(string)
	err = c.post(TxRouteName, bz, hash)
	return
}

func (c *Client) url(routeName string) string {
	fmt.Println(c.rpcURL + colon + c.rpcPort + router[routeName].Path)
	return c.rpcURL + colon + c.rpcPort + router[routeName].Path
}

func (c *Client) post(routeName string, json []byte, ptr any) lib.ErrorI {
	resp, err := c.client.Post(c.url(routeName), ApplicationJSON, bytes.NewBuffer(json))
	if err != nil {
		return ErrPostRequest(err)
	}
	return c.unmarshal(resp, ptr)
}

func (c *Client) get(routeName string, ptr any) lib.ErrorI {
	resp, err := c.client.Get(c.url(routeName))
	if err != nil {
		return ErrGetRequest(err)
	}
	return c.unmarshal(resp, ptr)
}

func (c *Client) unmarshal(resp *http.Response, ptr any) lib.ErrorI {
	bz, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrReadBody(err)
	}
	if resp.StatusCode != http.StatusOK {
		return ErrHttpStatus(resp.Status, resp.StatusCode, bz)
	}
	return lib.JSONUnmarshal(bz, ptr)
}
