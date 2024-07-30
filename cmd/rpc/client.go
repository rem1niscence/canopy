package rpc

import (
	"bytes"
	"encoding/json"
	bus "github.com/ginchuco/ginchu/app"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/p2p"
	"io"
	"net/http"
)

type Client struct {
	rpcURL    string
	rpcPort   string
	adminPort string
	client    http.Client
}

func NewClient(rpcURL, rpcPort, adminPort string) *Client {
	return &Client{rpcURL: rpcURL, rpcPort: rpcPort, adminPort: adminPort, client: http.Client{}}
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

func (c *Client) Proposals() (p *types.Proposals, err lib.ErrorI) {
	p = new(types.Proposals)
	err = c.get(ProposalsRouteName, p)
	return
}

func (c *Client) Poll() (p *types.Poll, err lib.ErrorI) {
	p = new(types.Poll)
	err = c.get(PollRouteName, p)
	return
}

func (c *Client) AddVote(proposal json.RawMessage, approve bool) (p *voteRequest, err lib.ErrorI) {
	p = new(voteRequest)
	bz, err := lib.MarshalJSON(voteRequest{
		Approve:  approve,
		Proposal: proposal,
	})
	if err != nil {
		return nil, err
	}
	err = c.post(AddVoteRouteName, bz, p, true)
	return
}

func (c *Client) DelVote(hash string) (p *hashRequest, err lib.ErrorI) {
	p = new(hashRequest)
	err = c.hashRequest(DelVoteRouteName, hash, p, true)
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

func (c *Client) Validators(height uint64, params lib.PageParams, filter lib.ValidatorFilters) (p *lib.Page, err lib.ErrorI) {
	p = new(lib.Page)
	err = c.paginatedHeightRequest(ValidatorsRouteName, height, params, p, filter)
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

func (c *Client) StateDiff(height, startHeight uint64) (diff string, err lib.ErrorI) {
	bz, err := lib.MarshalJSON(heightsRequest{heightRequest: heightRequest{height}, StartHeight: startHeight})
	if err != nil {
		return
	}
	resp, e := c.client.Post(c.url(StateDiffRouteName, false), ApplicationJSON, bytes.NewBuffer(bz))
	if e != nil {
		return "", ErrPostRequest(e)
	}
	bz, e = io.ReadAll(resp.Body)
	if e != nil {
		return "", ErrReadBody(e)
	}
	diff = string(bz)
	return
}

func (c *Client) TransactionJSON(tx json.RawMessage) (hash *string, err lib.ErrorI) {
	hash = new(string)
	err = c.post(TxRouteName, tx, hash)
	return
}

func (c *Client) Transaction(tx lib.TransactionI) (hash *string, err lib.ErrorI) {
	bz, err := lib.MarshalJSON(tx)
	if err != nil {
		return nil, err
	}
	hash = new(string)
	err = c.post(TxRouteName, bz, hash)
	return
}

func (c *Client) Keystore() (keystore *crypto.Keystore, err lib.ErrorI) {
	keystore = new(crypto.Keystore)
	err = c.get(KeystoreRouteName, keystore, true)
	return
}
func (c *Client) KeystoreNewKey(password string) (address crypto.AddressI, err lib.ErrorI) {
	address = new(crypto.Address)
	err = c.keystoreRequest(KeystoreNewKeyRouteName, keystoreRequest{
		passwordRequest: passwordRequest{password},
	}, address)
	return
}

func (c *Client) KeystoreImport(address string, epk crypto.EncryptedPrivateKey) (returned crypto.AddressI, err lib.ErrorI) {
	bz, err := lib.NewHexBytesFromString(address)
	if err != nil {
		return nil, err
	}
	returned = new(crypto.Address)
	err = c.keystoreRequest(KeystoreImportRouteName, keystoreRequest{
		addressRequest:      addressRequest{Address: bz},
		EncryptedPrivateKey: epk,
	}, returned)
	return
}

func (c *Client) KeystoreImportRaw(privateKey, password string) (returned crypto.AddressI, err lib.ErrorI) {
	bz, err := lib.NewHexBytesFromString(privateKey)
	if err != nil {
		return nil, err
	}
	returned = new(crypto.Address)
	err = c.keystoreRequest(KeystoreImportRawRouteName, keystoreRequest{
		PrivateKey:      bz,
		passwordRequest: passwordRequest{Password: password},
	}, returned)
	return
}

func (c *Client) KeystoreDelete(address string) (returned crypto.AddressI, err lib.ErrorI) {
	bz, err := lib.NewHexBytesFromString(address)
	if err != nil {
		return nil, err
	}
	returned = new(crypto.Address)
	err = c.keystoreRequest(KeystoreDeleteRouteName, keystoreRequest{
		addressRequest: addressRequest{bz},
	}, returned)
	return
}

func (c *Client) KeystoreGet(address, password string) (returned *crypto.KeyGroup, err lib.ErrorI) {
	bz, err := lib.NewHexBytesFromString(address)
	if err != nil {
		return nil, err
	}
	returned = new(crypto.KeyGroup)
	err = c.keystoreRequest(KeystoreGetRouteName, keystoreRequest{
		addressRequest:  addressRequest{bz},
		passwordRequest: passwordRequest{password},
	}, returned)
	return
}

func (c *Client) TxSend(from, rec string, amt uint64, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	fromHex, err := lib.NewHexBytesFromString(from)
	if err != nil {
		return nil, nil, err
	}
	return c.transactionRequest(TxSendRouteName, txRequest{
		Amount:          amt,
		Output:          rec,
		Sequence:        optSeq,
		Fee:             optFee,
		Submit:          submit,
		addressRequest:  addressRequest{Address: fromHex},
		passwordRequest: passwordRequest{Password: pwd},
	})
}

func (c *Client) TxStake(from, netAddr string, amt uint64, output, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	return c.txStake(from, netAddr, amt, output, pwd, submit, false, optSeq, optFee)
}

func (c *Client) TxEditStake(from, netAddr string, amt uint64, output, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	return c.txStake(from, netAddr, amt, output, pwd, submit, true, optSeq, optFee)
}

func (c *Client) TxUnstake(from, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	return c.txAddress(TxUnstakeRouteName, from, pwd, submit, optSeq, optFee)
}

func (c *Client) TxPause(from, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	return c.txAddress(TxPauseRouteName, from, pwd, submit, optSeq, optFee)
}

func (c *Client) TxUnpause(from, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	return c.txAddress(TxUnpauseRouteName, from, pwd, submit, optSeq, optFee)
}

func (c *Client) TxChangeParam(from, pSpace, pKey, pValue string, startBlk, endBlk uint64,
	pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	fromHex, err := lib.NewHexBytesFromString(from)
	if err != nil {
		return nil, nil, err
	}
	return c.transactionRequest(TxChangeParamRouteName, txRequest{
		Sequence:        optSeq,
		Fee:             optFee,
		Submit:          submit,
		addressRequest:  addressRequest{Address: fromHex},
		passwordRequest: passwordRequest{Password: pwd},
		txChangeParamRequest: txChangeParamRequest{
			ParamSpace: pSpace,
			ParamKey:   pKey,
			ParamValue: pValue,
			StartBlock: startBlk,
			EndBlock:   endBlk,
		},
	})
}

func (c *Client) TxDaoTransfer(from string, amt, startBlk, endBlk uint64,
	pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	fromHex, err := lib.NewHexBytesFromString(from)
	if err != nil {
		return nil, nil, err
	}
	return c.transactionRequest(TxDAOTransferRouteName, txRequest{
		Amount:          amt,
		Sequence:        optSeq,
		Fee:             optFee,
		Submit:          submit,
		addressRequest:  addressRequest{Address: fromHex},
		passwordRequest: passwordRequest{Password: pwd},
		txChangeParamRequest: txChangeParamRequest{
			StartBlock: startBlk,
			EndBlock:   endBlk,
		},
	})
}

func (c *Client) ResourceUsage() (returned *resourceUsageResponse, err lib.ErrorI) {
	returned = new(resourceUsageResponse)
	err = c.get(ResourceUsageRouteName, returned, true)
	return
}

func (c *Client) PeerInfo() (returned *peerInfoResponse, err lib.ErrorI) {
	returned = new(peerInfoResponse)
	err = c.get(PeerInfoRouteName, returned, true)
	return
}

func (c *Client) ConsensusInfo() (returned *bus.ConsensusSummary, err lib.ErrorI) {
	returned = new(bus.ConsensusSummary)
	err = c.get(ConsensusInfoRouteName, returned, true)
	return
}

func (c *Client) PeerBook() (returned *[]*p2p.BookPeer, err lib.ErrorI) {
	returned = new([]*p2p.BookPeer)
	err = c.get(PeerBookRouteName, returned, true)
	return
}

func (c *Client) Config() (returned *lib.Config, err lib.ErrorI) {
	returned = new(lib.Config)
	err = c.get(ConfigRouteName, returned, true)
	return
}

func (c *Client) Logs() (logs string, err lib.ErrorI) {
	resp, e := c.client.Get(c.url(LogsRouteName, true))
	if e != nil {
		return "", ErrGetRequest(err)
	}
	bz, e := io.ReadAll(resp.Body)
	if e != nil {
		return "", ErrGetRequest(e)
	}
	return string(bz), nil
}

func (c *Client) txAddress(route string, from, pwd string, submit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	fromHex, err := lib.NewHexBytesFromString(from)
	if err != nil {
		return nil, nil, err
	}
	return c.transactionRequest(route, txRequest{
		Sequence:        optSeq,
		Fee:             optFee,
		Submit:          submit,
		addressRequest:  addressRequest{Address: fromHex},
		passwordRequest: passwordRequest{Password: pwd},
	})
}

func (c *Client) txStake(from, netAddr string, amt uint64, output, pwd string, submit, edit bool, optSeq, optFee uint64) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	route := TxStakeRouteName
	if edit {
		route = TxEditStakeRouteName
	}
	fromHex, err := lib.NewHexBytesFromString(from)
	if err != nil {
		return nil, nil, err
	}
	return c.transactionRequest(route, txRequest{
		Amount:          amt,
		NetAddress:      netAddr,
		Output:          output,
		Sequence:        optSeq,
		Fee:             optFee,
		Submit:          submit,
		addressRequest:  addressRequest{Address: fromHex},
		passwordRequest: passwordRequest{Password: pwd},
	})
}

func (c *Client) transactionRequest(routeName string, txRequest txRequest) (hash *string, tx json.RawMessage, e lib.ErrorI) {
	bz, e := lib.MarshalJSON(txRequest)
	if e != nil {
		return
	}
	if txRequest.Submit {
		hash = new(string)
		e = c.post(routeName, bz, hash, true)
	} else {
		tx = json.RawMessage{}
		e = c.post(routeName, bz, &tx, true)
	}
	return
}

func (c *Client) keystoreRequest(routeName string, keystoreRequest keystoreRequest, ptr any) (err lib.ErrorI) {
	bz, err := lib.MarshalJSON(keystoreRequest)
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr, true)
	return
}

func (c *Client) paginatedHeightRequest(routeName string, height uint64, p lib.PageParams, ptr any, filters ...lib.ValidatorFilters) (err lib.ErrorI) {
	var vf lib.ValidatorFilters
	if filters != nil {
		vf = filters[0]
	}
	bz, err := lib.MarshalJSON(paginatedHeightRequest{
		heightRequest:    heightRequest{height},
		PageParams:       p,
		ValidatorFilters: vf,
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
	bz, err := lib.MarshalJSON(paginatedAddressRequest{
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
	bz, err := lib.MarshalJSON(heightRequest{Height: height})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) hashRequest(routeName string, hash string, ptr any, admin ...bool) (err lib.ErrorI) {
	bz, err := lib.MarshalJSON(hashRequest{Hash: hash})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr, admin...)
	return
}

func (c *Client) heightAndAddressRequest(routeName string, height uint64, address string, ptr any) (err lib.ErrorI) {
	addr, err := lib.StringToBytes(address)
	if err != nil {
		return err
	}
	bz, err := lib.MarshalJSON(heightAndAddressRequest{
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
	bz, err := lib.MarshalJSON(heightAndNameRequest{
		heightRequest: heightRequest{height},
		nameRequest:   nameRequest{name},
	})
	if err != nil {
		return
	}
	err = c.post(routeName, bz, ptr)
	return
}

func (c *Client) url(routeName string, admin ...bool) string {
	if admin != nil && admin[0] {
		return "http://" + localhost + colon + c.adminPort + router[routeName].Path
	}
	return c.rpcURL + colon + c.rpcPort + router[routeName].Path
}

func (c *Client) post(routeName string, json []byte, ptr any, admin ...bool) lib.ErrorI {
	resp, err := c.client.Post(c.url(routeName, admin...), ApplicationJSON, bytes.NewBuffer(json))
	if err != nil {
		return ErrPostRequest(err)
	}
	return c.unmarshal(resp, ptr)
}

func (c *Client) get(routeName string, ptr any, admin ...bool) lib.ErrorI {
	resp, err := c.client.Get(c.url(routeName, admin...))
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
	return lib.UnmarshalJSON(bz, ptr)
}
