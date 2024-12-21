package rpc

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	app2 "github.com/canopy-network/canopy/controller"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/store"
	"github.com/dgraph-io/badger/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/nsf/jsondiff"
	"github.com/rs/cors"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"io"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	pprof2 "runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ContentType     = "Content-MessageType"
	ApplicationJSON = "application/json; charset=utf-8"
	localhost       = "127.0.0.1"
	colon           = ":"

	VersionRouteName              = "version"
	TxRouteName                   = "tx"
	HeightRouteName               = "height"
	AccountRouteName              = "account"
	AccountsRouteName             = "accounts"
	PoolRouteName                 = "pool"
	PoolsRouteName                = "pools"
	ValidatorRouteName            = "validator"
	ValidatorsRouteName           = "validators"
	NonSignersRouteName           = "non-signers"
	SupplyRouteName               = "supply"
	ParamRouteName                = "params"
	FeeParamRouteName             = "fee-params"
	GovParamRouteName             = "gov-params"
	ConParamsRouteName            = "con-params"
	ValParamRouteName             = "val-params"
	StateRouteName                = "state"
	StateDiffRouteName            = "state-diff"
	StateDiffGetRouteName         = "state-diff-get"
	CertByHeightRouteName         = "cert-by-height"
	BlocksRouteName               = "blocks"
	BlockByHeightRouteName        = "block-by-height"
	BlockByHashRouteName          = "block-by-hash"
	TxsByHeightRouteName          = "txs-by-height"
	TxsBySenderRouteName          = "txs-by-sender"
	TxsByRecRouteName             = "txs-by-rec"
	TxByHashRouteName             = "tx-by-hash"
	PendingRouteName              = "pending"
	ProposalsRouteName            = "proposals"
	PollRouteName                 = "poll"
	CommitteeRouteName            = "committee"
	CommitteeDataRouteName        = "committee-data"
	CommitteesDataRouteName       = "committees-data"
	SubsidizedCommitteesRouteName = "subsidized-committees"
	OrderRouteName                = "order"
	OrdersRouteName               = "orders"
	// debug
	DebugBlockedRouteName = "blocked"
	DebugHeapRouteName    = "heap"
	DebugCPURouteName     = "cpu"
	DebugRoutineRouteName = "routine"
	// admin
	KeystoreRouteName          = "keystore"
	KeystoreNewKeyRouteName    = "keystore-new-key"
	KeystoreImportRouteName    = "keystore-import"
	KeystoreImportRawRouteName = "keystore-import-raw"
	KeystoreDeleteRouteName    = "keystore-delete"
	KeystoreGetRouteName       = "keystore-get"
	TxSendRouteName            = "tx-send"
	TxStakeRouteName           = "tx-stake"
	TxUnstakeRouteName         = "tx-unstake"
	TxEditStakeRouteName       = "tx-edit-stake"
	TxPauseRouteName           = "tx-pause"
	TxUnpauseRouteName         = "tx-unpause"
	TxChangeParamRouteName     = "tx-change-param"
	TxDAOTransferRouteName     = "tx-dao-transfer"
	TxSubsidyRouteName         = "tx-subsidy"
	TxCreateOrderRouteName     = "tx-create-order"
	TxEditOrderRouteName       = "tx-edit-order"
	TxDeleteOrderRouteName     = "tx-delete-order"
	ResourceUsageRouteName     = "resource-usage"
	PeerInfoRouteName          = "peer-info"
	ConsensusInfoRouteName     = "consensus-info"
	PeerBookRouteName          = "peer-book"
	ConfigRouteName            = "config"
	LogsRouteName              = "logs"
	AddVoteRouteName           = "add-vote"
	DelVoteRouteName           = "del-vote"
	ExplorerRouteName          = "explorer"
	WalletRouteName            = "wallet"
)

const SoftwareVersion = "0.0.0-alpha"

var (
	app    *app2.Controller
	db     *badger.DB
	conf   lib.Config
	logger lib.LoggerI

	router = routes{
		VersionRouteName:              {Method: http.MethodGet, Path: "/v1/", HandlerFunc: Version},
		TxRouteName:                   {Method: http.MethodPost, Path: "/v1/tx", HandlerFunc: Transaction},
		HeightRouteName:               {Method: http.MethodPost, Path: "/v1/query/height", HandlerFunc: Height},
		AccountRouteName:              {Method: http.MethodPost, Path: "/v1/query/account", HandlerFunc: Account},
		AccountsRouteName:             {Method: http.MethodPost, Path: "/v1/query/accounts", HandlerFunc: Accounts},
		PoolRouteName:                 {Method: http.MethodPost, Path: "/v1/query/pool", HandlerFunc: Pool},
		PoolsRouteName:                {Method: http.MethodPost, Path: "/v1/query/pools", HandlerFunc: Pools},
		ValidatorRouteName:            {Method: http.MethodPost, Path: "/v1/query/validator", HandlerFunc: Validator},
		ValidatorsRouteName:           {Method: http.MethodPost, Path: "/v1/query/validators", HandlerFunc: Validators},
		CommitteeRouteName:            {Method: http.MethodPost, Path: "/v1/query/committee", HandlerFunc: Committee},
		CommitteeDataRouteName:        {Method: http.MethodPost, Path: "/v1/query/committee-data", HandlerFunc: CommitteeData},
		CommitteesDataRouteName:       {Method: http.MethodPost, Path: "/v1/query/committees-data", HandlerFunc: CommitteesData},
		SubsidizedCommitteesRouteName: {Method: http.MethodPost, Path: "/v1/query/subsidized-committees", HandlerFunc: SubsidizedCommittees},
		NonSignersRouteName:           {Method: http.MethodPost, Path: "/v1/query/non-signers", HandlerFunc: NonSigners},
		ParamRouteName:                {Method: http.MethodPost, Path: "/v1/query/params", HandlerFunc: Params},
		SupplyRouteName:               {Method: http.MethodPost, Path: "/v1/query/supply", HandlerFunc: Supply},
		FeeParamRouteName:             {Method: http.MethodPost, Path: "/v1/query/fee-params", HandlerFunc: FeeParams},
		GovParamRouteName:             {Method: http.MethodPost, Path: "/v1/query/gov-params", HandlerFunc: GovParams},
		ConParamsRouteName:            {Method: http.MethodPost, Path: "/v1/query/con-params", HandlerFunc: ConParams},
		ValParamRouteName:             {Method: http.MethodPost, Path: "/v1/query/val-params", HandlerFunc: ValParams},
		StateRouteName:                {Method: http.MethodPost, Path: "/v1/query/state", HandlerFunc: State},
		StateDiffRouteName:            {Method: http.MethodPost, Path: "/v1/query/state-diff", HandlerFunc: StateDiff},
		StateDiffGetRouteName:         {Method: http.MethodGet, Path: "/v1/query/state-diff", HandlerFunc: StateDiff},
		CertByHeightRouteName:         {Method: http.MethodPost, Path: "/v1/query/cert-by-height", HandlerFunc: CertByHeight},
		BlockByHeightRouteName:        {Method: http.MethodPost, Path: "/v1/query/block-by-height", HandlerFunc: BlockByHeight},
		BlocksRouteName:               {Method: http.MethodPost, Path: "/v1/query/blocks", HandlerFunc: Blocks},
		BlockByHashRouteName:          {Method: http.MethodPost, Path: "/v1/query/block-by-hash", HandlerFunc: BlockByHash},
		TxsByHeightRouteName:          {Method: http.MethodPost, Path: "/v1/query/txs-by-height", HandlerFunc: TransactionsByHeight},
		TxsBySenderRouteName:          {Method: http.MethodPost, Path: "/v1/query/txs-by-sender", HandlerFunc: TransactionsBySender},
		TxsByRecRouteName:             {Method: http.MethodPost, Path: "/v1/query/txs-by-rec", HandlerFunc: TransactionsByRecipient},
		TxByHashRouteName:             {Method: http.MethodPost, Path: "/v1/query/tx-by-hash", HandlerFunc: TransactionByHash},
		OrderRouteName:                {Method: http.MethodPost, Path: "/v1/query/order", HandlerFunc: Order},
		OrdersRouteName:               {Method: http.MethodPost, Path: "/v1/query/orders", HandlerFunc: Orders},
		PendingRouteName:              {Method: http.MethodPost, Path: "/v1/query/pending", HandlerFunc: Pending},
		ProposalsRouteName:            {Method: http.MethodGet, Path: "/v1/gov/proposals", HandlerFunc: Proposals},
		PollRouteName:                 {Method: http.MethodGet, Path: "/v1/gov/poll", HandlerFunc: Poll},
		// debug
		DebugBlockedRouteName: {Method: http.MethodPost, Path: "/debug/blocked", HandlerFunc: debugHandler(DebugBlockedRouteName)},
		DebugHeapRouteName:    {Method: http.MethodPost, Path: "/debug/heap", HandlerFunc: debugHandler(DebugHeapRouteName)},
		DebugCPURouteName:     {Method: http.MethodPost, Path: "/debug/cpu", HandlerFunc: debugHandler(DebugHeapRouteName)},
		DebugRoutineRouteName: {Method: http.MethodPost, Path: "/debug/routine", HandlerFunc: debugHandler(DebugRoutineRouteName)},
		// admin
		KeystoreRouteName:          {Method: http.MethodGet, Path: "/v1/admin/keystore", HandlerFunc: Keystore, AdminOnly: true},
		KeystoreNewKeyRouteName:    {Method: http.MethodPost, Path: "/v1/admin/keystore-new-key", HandlerFunc: KeystoreNewKey, AdminOnly: true},
		KeystoreImportRouteName:    {Method: http.MethodPost, Path: "/v1/admin/keystore-import", HandlerFunc: KeystoreImport, AdminOnly: true},
		KeystoreImportRawRouteName: {Method: http.MethodPost, Path: "/v1/admin/keystore-import-raw", HandlerFunc: KeystoreImportRaw, AdminOnly: true},
		KeystoreDeleteRouteName:    {Method: http.MethodPost, Path: "/v1/admin/keystore-delete", HandlerFunc: KeystoreDelete, AdminOnly: true},
		KeystoreGetRouteName:       {Method: http.MethodPost, Path: "/v1/admin/keystore-get", HandlerFunc: KeystoreGetKeyGroup, AdminOnly: true},
		TxSendRouteName:            {Method: http.MethodPost, Path: "/v1/admin/tx-send", HandlerFunc: TransactionSend, AdminOnly: true},
		TxStakeRouteName:           {Method: http.MethodPost, Path: "/v1/admin/tx-stake", HandlerFunc: TransactionStake, AdminOnly: true},
		TxEditStakeRouteName:       {Method: http.MethodPost, Path: "/v1/admin/tx-edit-stake", HandlerFunc: TransactionEditStake, AdminOnly: true},
		TxUnstakeRouteName:         {Method: http.MethodPost, Path: "/v1/admin/tx-unstake", HandlerFunc: TransactionUnstake, AdminOnly: true},
		TxPauseRouteName:           {Method: http.MethodPost, Path: "/v1/admin/tx-pause", HandlerFunc: TransactionPause, AdminOnly: true},
		TxUnpauseRouteName:         {Method: http.MethodPost, Path: "/v1/admin/tx-unpause", HandlerFunc: TransactionUnpause, AdminOnly: true},
		TxChangeParamRouteName:     {Method: http.MethodPost, Path: "/v1/admin/tx-change-param", HandlerFunc: TransactionChangeParam, AdminOnly: true},
		TxDAOTransferRouteName:     {Method: http.MethodPost, Path: "/v1/admin/tx-dao-transfer", HandlerFunc: TransactionDAOTransfer, AdminOnly: true},
		TxCreateOrderRouteName:     {Method: http.MethodPost, Path: "/v1/admin/tx-create-order", HandlerFunc: TransactionCreateOrder, AdminOnly: true},
		TxEditOrderRouteName:       {Method: http.MethodPost, Path: "/v1/admin/tx-edit-order", HandlerFunc: TransactionEditOrder, AdminOnly: true},
		TxDeleteOrderRouteName:     {Method: http.MethodPost, Path: "/v1/admin/tx-delete-order", HandlerFunc: TransactionDeleteOrder, AdminOnly: true},
		TxSubsidyRouteName:         {Method: http.MethodPost, Path: "/v1/admin/subsidy", HandlerFunc: TransactionSubsidy, AdminOnly: true},
		ResourceUsageRouteName:     {Method: http.MethodGet, Path: "/v1/admin/resource-usage", HandlerFunc: ResourceUsage, AdminOnly: true},
		PeerInfoRouteName:          {Method: http.MethodGet, Path: "/v1/admin/peer-info", HandlerFunc: PeerInfo, AdminOnly: true},
		ConsensusInfoRouteName:     {Method: http.MethodGet, Path: "/v1/admin/consensus-info", HandlerFunc: ConsensusInfo, AdminOnly: true},
		PeerBookRouteName:          {Method: http.MethodGet, Path: "/v1/admin/peer-book", HandlerFunc: PeerBook, AdminOnly: true},
		ConfigRouteName:            {Method: http.MethodGet, Path: "/v1/admin/config", HandlerFunc: Config, AdminOnly: true},
		LogsRouteName:              {Method: http.MethodGet, Path: "/v1/admin/log", HandlerFunc: logsHandler(), AdminOnly: true},
		AddVoteRouteName:           {Method: http.MethodPost, Path: "/v1/gov/add-vote", HandlerFunc: AddVote, AdminOnly: true},
		DelVoteRouteName:           {Method: http.MethodPost, Path: "/v1/gov/del-vote", HandlerFunc: DelVote, AdminOnly: true},
	}
)

//go:embed all:web/explorer/out
var explorerFS embed.FS

//go:embed all:web/wallet/out
var walletFS embed.FS

const (
	walletStaticDir   = "web/wallet/out"
	explorerStaticDir = "web/explorer/out"
)

func StartRPC(a *app2.Controller, c lib.Config, l lib.LoggerI) {
	cor := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:" + c.WalletPort, "http://localhost:" + c.ExplorerPort},
		AllowedMethods: []string{"GET", "OPTIONS", "POST"},
	})
	s, timeout := a.CanopyFSM().Store().(lib.StoreI), time.Duration(c.TimeoutS)*time.Second
	app, conf, db, logger = a, c, s.DB(), l
	l.Infof("Starting RPC server at 0.0.0.0:%s", c.RPCPort)
	go func() {
		l.Fatal((&http.Server{
			Addr:    colon + c.RPCPort,
			Handler: cor.Handler(http.TimeoutHandler(router.New(), timeout, ErrServerTimeout().Error())),
		}).ListenAndServe().Error())
	}()
	l.Infof("Starting Admin RPC server at %s:%s", localhost, c.AdminPort)
	go func() {
		l.Fatal((&http.Server{
			Addr:    localhost + colon + c.AdminPort,
			Handler: cor.Handler(http.TimeoutHandler(router.NewAdmin(), timeout, ErrServerTimeout().Error())),
		}).ListenAndServe().Error())
	}()
	go func() {
		fileName := "heap1.out"
		for range time.Tick(time.Second * 10) {
			f, err := os.Create(filepath.Join(c.DataDirPath, fileName))
			if err != nil {
				l.Fatalf("could not create memory profile: ", err)
			}
			runtime.GC() // get up-to-date statistics
			if err = pprof2.WriteHeapProfile(f); err != nil {
				l.Fatalf("could not write memory profile: ", err)
			}
			f.Close()
			fileName = "heap2.out"
		}
	}()
	//l.Infof("Starting Web Wallet 🔑 http://localhost:%s ⬅️", c.WalletPort)
	//runStaticFileServer(walletFS, walletStaticDir, c.WalletPort)
	//l.Infof("Starting Block Explorer 🔍️ http://localhost:%s ⬅️", c.ExplorerPort)
	//runStaticFileServer(explorerFS, explorerStaticDir, c.ExplorerPort)
	//go pollValidators(time.Minute)
}

func Version(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	write(w, SoftwareVersion, http.StatusOK)
}

func Height(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	state, ok := getStateMachineWithHeight(0, w)
	if !ok {
		return
	}
	write(w, state.Height(), http.StatusOK)
}

func BlockByHeight(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightIndexer(w, r, func(s lib.StoreI, h uint64, _ lib.PageParams) (any, lib.ErrorI) { return s.GetBlockByHeight(h) })
}

func CertByHeight(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightIndexer(w, r, func(s lib.StoreI, h uint64, _ lib.PageParams) (any, lib.ErrorI) { return s.GetQCByHeight(h) })
}

func BlockByHash(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	hashIndexer(w, r, func(s lib.StoreI, h lib.HexBytes) (any, lib.ErrorI) { return s.GetBlockByHash(h) })
}

func Blocks(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightIndexer(w, r, func(s lib.StoreI, _ uint64, p lib.PageParams) (any, lib.ErrorI) { return s.GetBlocks(p) })
}

func TransactionByHash(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	hashIndexer(w, r, func(s lib.StoreI, h lib.HexBytes) (any, lib.ErrorI) { return s.GetTxByHash(h) })
}

func TransactionsByHeight(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightIndexer(w, r, func(s lib.StoreI, h uint64, p lib.PageParams) (any, lib.ErrorI) { return s.GetTxsByHeight(h, true, p) })
}

func Pending(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	addrIndexer(w, r, func(_ lib.StoreI, _ crypto.AddressI, p lib.PageParams) (any, lib.ErrorI) {
		return app.GetPendingPage(p)
	})
}

func Proposals(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bz, err := os.ReadFile(filepath.Join(conf.DataDirPath, lib.ProposalsFilePath))
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set(ContentType, ApplicationJSON)
	if _, err = w.Write(bz); err != nil {
		logger.Error(err.Error())
	}
}

var (
	pollMux = sync.Mutex{}
	poll    = make(types.Poll)
)

func Poll(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	pollMux.Lock()
	bz, e := lib.MarshalJSONIndent(poll)
	pollMux.Unlock()
	if e != nil {
		write(w, e, http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(bz); err != nil {
		logger.Error(err.Error())
	}
}

func pollValidators(frequency time.Duration) {
	s, e := store.NewStoreWithDB(db, logger)
	if e != nil {
		panic(e)
	}
	defer s.Discard()
	for {
		state, err := fsm.New(conf, s, logger)
		if err != nil {
			logger.Error(err.Error())
			time.Sleep(frequency)
			continue
		}
		vals, err := state.GetCanopyCommitteeMembers()
		if err != nil {
			logger.Error(err.Error())
			time.Sleep(frequency)
			continue
		}
		pollMux.Lock()
		poll = types.PollValidators(vals, router[ProposalsRouteName].Path, logger)
		pollMux.Unlock()
		time.Sleep(frequency)
	}
}

func AddVote(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	proposals := make(types.GovProposals)
	if err := proposals.NewFromFile(conf.DataDirPath); err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	j := new(voteRequest)
	if !unmarshal(w, r, j) {
		return
	}
	prop, err := types.NewProposalFromBytes(j.Proposal)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	if err = proposals.Add(prop, j.Approve); err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	if err = proposals.SaveToFile(conf.DataDirPath); err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	write(w, j, http.StatusOK)
}

func DelVote(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	proposals := make(types.GovProposals)
	if err := proposals.NewFromFile(conf.DataDirPath); err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	j := new(voteRequest)
	if !unmarshal(w, r, j) {
		return
	}
	prop, err := types.NewProposalFromBytes(j.Proposal)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	proposals.Del(prop)
	if err = proposals.SaveToFile(conf.DataDirPath); err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	write(w, j, http.StatusOK)
}

func TransactionsBySender(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	addrIndexer(w, r, func(s lib.StoreI, a crypto.AddressI, p lib.PageParams) (any, lib.ErrorI) {
		return s.GetTxsBySender(a, true, p)
	})
}

func TransactionsByRecipient(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	addrIndexer(w, r, func(s lib.StoreI, a crypto.AddressI, p lib.PageParams) (any, lib.ErrorI) {
		return s.GetTxsByRecipient(a, true, p)
	})
}

func Account(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightAndAddressParams(w, r, func(s *fsm.StateMachine, a lib.HexBytes) (interface{}, lib.ErrorI) {
		return s.GetAccount(crypto.NewAddressFromBytes(a))
	})
}

func Accounts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p *paginatedHeightRequest) (interface{}, lib.ErrorI) {
		return s.GetAccountsPaginated(p.PageParams)
	})
}

func Pool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightAndIdParams(w, r, func(s *fsm.StateMachine, id uint64) (interface{}, lib.ErrorI) {
		return s.GetPool(id)
	})
}

func Pools(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p *paginatedHeightRequest) (interface{}, lib.ErrorI) {
		return s.GetPoolsPaginated(p.PageParams)
	})
}

func NonSigners(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (interface{}, lib.ErrorI) {
		return s.GetNonSigners()
	})
}

func Supply(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (interface{}, lib.ErrorI) {
		return s.GetSupply()
	})
}

func Validator(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightAndAddressParams(w, r, func(s *fsm.StateMachine, a lib.HexBytes) (interface{}, lib.ErrorI) {
		return s.GetValidator(crypto.NewAddressFromBytes(a))
	})
}

func Validators(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p *paginatedHeightRequest) (interface{}, lib.ErrorI) {
		return s.GetValidatorsPaginated(p.PageParams, p.ValidatorFilters)
	})
}

func Committee(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p *paginatedHeightRequest) (interface{}, lib.ErrorI) {
		return s.GetCommitteePaginated(p.PageParams, p.ValidatorFilters.Committee)
	})
}

func CommitteeData(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightAndIdParams(w, r, func(s *fsm.StateMachine, id uint64) (interface{}, lib.ErrorI) {
		return s.GetCommitteeData(id)
	})
}

func CommitteesData(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p *paginatedHeightRequest) (interface{}, lib.ErrorI) {
		return s.GetCommitteesData() // consider pagination
	})
}

func SubsidizedCommittees(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (interface{}, lib.ErrorI) { return s.GetSubsidizedCommittees() })
}

func Order(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	orderParams(w, r, func(s *fsm.StateMachine, p *orderRequest) (any, lib.ErrorI) {
		return s.GetOrder(p.OrderId, p.CommitteeId)
	})
}

func Orders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightAndIdParams(w, r, func(s *fsm.StateMachine, id uint64) (any, lib.ErrorI) { return s.GetOrderBook(id) })
}

func Params(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (interface{}, lib.ErrorI) { return s.GetParams() })
}

func FeeParams(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (any, lib.ErrorI) { return s.GetParamsFee() })
}

func ValParams(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (any, lib.ErrorI) { return s.GetParamsVal() })
}

func ConParams(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (any, lib.ErrorI) { return s.GetParamsCons() })
}

func GovParams(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (any, lib.ErrorI) { return s.GetParamsGov() })
}

func State(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightParams(w, r, func(s *fsm.StateMachine) (any, lib.ErrorI) { return s.ExportState() })
}

func StateDiff(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sm1, sm2, opts, ok := getDoubleStateMachineFromHeightParams(w, r, p)
	if !ok {
		return
	}
	state1, e := sm1.ExportState()
	if e != nil {
		write(w, e.Error(), http.StatusInternalServerError)
		return
	}
	state2, e := sm2.ExportState()
	if e != nil {
		write(w, e.Error(), http.StatusInternalServerError)
		return
	}
	j1, _ := json.Marshal(state1)
	j2, _ := json.Marshal(state2)
	_, differ := jsondiff.Compare(j1, j2, opts)
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		differ = "<pre>" + differ + "</pre>"
	}
	if _, err := w.Write([]byte(differ)); err != nil {
		logger.Error(err.Error())
	}
}

func Transaction(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tx := new(lib.Transaction)
	if ok := unmarshal(w, r, tx); !ok {
		return
	}
	submitTx(w, tx)
}

func Keystore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keystore, err := crypto.NewKeystoreFromFile(conf.DataDirPath)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, keystore, http.StatusOK)
}

func KeystoreNewKey(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keystoreHandler(w, r, func(k *crypto.Keystore, ptr *keystoreRequest) (any, error) {
		pk, err := crypto.NewBLS12381PrivateKey()
		if err != nil {
			return nil, err
		}
		address, err := k.ImportRaw(pk.Bytes(), ptr.Password)
		if err != nil {
			return nil, err
		}
		return address, k.SaveToFile(conf.DataDirPath)
	})
}

func KeystoreImport(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keystoreHandler(w, r, func(k *crypto.Keystore, ptr *keystoreRequest) (any, error) {
		if err := k.Import(ptr.Address, &ptr.EncryptedPrivateKey); err != nil {
			return nil, err
		}
		return ptr.Address, k.SaveToFile(conf.DataDirPath)
	})
}

func KeystoreImportRaw(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keystoreHandler(w, r, func(k *crypto.Keystore, ptr *keystoreRequest) (any, error) {
		address, err := k.ImportRaw(ptr.PrivateKey, ptr.Password)
		if err != nil {
			return nil, err
		}
		return address, k.SaveToFile(conf.DataDirPath)
	})
}

func KeystoreDelete(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keystoreHandler(w, r, func(k *crypto.Keystore, ptr *keystoreRequest) (any, error) {
		k.DeleteKey(ptr.Address)
		return ptr.Address, k.SaveToFile(conf.DataDirPath)
	})
}

func KeystoreGetKeyGroup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keystoreHandler(w, r, func(k *crypto.Keystore, ptr *keystoreRequest) (any, error) {
		return k.GetKeyGroup(ptr.Address, ptr.Password)
	})
}

type txRequest struct {
	Amount          uint64       `json:"amount"`
	NetAddress      string       `json:"netAddress"`
	Output          string       `json:"output"`
	OpCode          string       `json:"opCode"`
	Fee             uint64       `json:"fee"`
	Delegate        bool         `json:"delegate"`
	EarlyWithdrawal bool         `json:"earlyWithdrawal"`
	Submit          bool         `json:"submit"`
	ReceiveAmount   uint64       `json:"receiveAmount"`
	ReceiveAddress  lib.HexBytes `json:"receiveAddress"`
	OrderId         uint64       `json:"orderId"`
	addressRequest
	passwordRequest
	txChangeParamRequest
	committeesRequest
}

type txChangeParamRequest struct {
	ParamSpace string `json:"paramSpace"`
	ParamKey   string `json:"paramKey"`
	ParamValue string `json:"paramValue"`
	StartBlock uint64 `json:"startBlock"`
	EndBlock   uint64 `json:"endBlock"`
}

func TransactionSend(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		toAddress, err := crypto.NewAddressFromString(ptr.Output)
		if err != nil {
			return nil, err
		}
		return types.NewSendTransaction(p, toAddress, ptr.Amount, ptr.Fee)
	})
}

func TransactionStake(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		outputAddress, err := crypto.NewAddressFromString(ptr.Output)
		if err != nil {
			return nil, err
		}
		committees, err := StringToCommittees(ptr.committees)
		if err != nil {
			return nil, err
		}
		return types.NewStakeTx(p, outputAddress, ptr.NetAddress, committees, ptr.Amount, ptr.Fee, ptr.Delegate, ptr.EarlyWithdrawal)
	})
}

func TransactionEditStake(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		outputAddress, err := crypto.NewAddressFromString(ptr.Output)
		if err != nil {
			return nil, err
		}
		committees, err := StringToCommittees(ptr.committees)
		if err != nil {
			return nil, err
		}
		return types.NewEditStakeTx(p, outputAddress, ptr.NetAddress, committees, ptr.Amount, ptr.Fee, ptr.EarlyWithdrawal)
	})
}

func TransactionUnstake(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		return types.NewUnstakeTx(p, ptr.Fee)
	})
}

func TransactionPause(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		return types.NewPauseTx(p, ptr.Fee)
	})
}

func TransactionUnpause(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		return types.NewUnpauseTx(p, ptr.Fee)
	})
}

func TransactionChangeParam(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		ptr.ParamSpace = types.FormatParamSpace(ptr.ParamSpace)
		isString, err := types.IsStringParam(ptr.ParamSpace, ptr.ParamKey)
		if err != nil {
			return nil, err
		}
		if isString {
			return types.NewChangeParamTxString(p, ptr.ParamSpace, ptr.ParamKey, ptr.ParamValue, ptr.StartBlock, ptr.EndBlock, ptr.Fee)
		} else {
			paramValue, err := strconv.ParseUint(ptr.ParamValue, 10, 64)
			if err != nil {
				return nil, err
			}
			return types.NewChangeParamTxUint64(p, ptr.ParamSpace, ptr.ParamKey, paramValue, ptr.StartBlock, ptr.EndBlock, ptr.Fee)
		}
	})
}

func TransactionDAOTransfer(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		return types.NewDAOTransferTx(p, ptr.Amount, ptr.StartBlock, ptr.EndBlock, ptr.Fee)
	})
}

func TransactionSubsidy(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		committeeId := uint64(0)
		if c, err := StringToCommittees(ptr.committees); err == nil {
			committeeId = c[0]
		}
		return types.NewSubsidyTx(p, ptr.Amount, committeeId, ptr.OpCode, ptr.Fee)
	})
}

func TransactionCreateOrder(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		committeeId := uint64(0)
		if c, err := StringToCommittees(ptr.committees); err == nil {
			committeeId = c[0]
		}
		return types.NewCreateOrderTx(p, ptr.Amount, ptr.ReceiveAmount, committeeId, ptr.ReceiveAddress, ptr.Fee)
	})
}

func TransactionEditOrder(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		committeeId := uint64(0)
		if c, err := StringToCommittees(ptr.committees); err == nil {
			committeeId = c[0]
		}
		return types.NewEditOrderTx(p, ptr.OrderId, ptr.Amount, ptr.ReceiveAmount, committeeId, ptr.ReceiveAddress, ptr.Fee)
	})
}

func TransactionDeleteOrder(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	txHandler(w, r, func(p crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error) {
		committeeId := uint64(0)
		if c, err := StringToCommittees(ptr.committees); err == nil {
			committeeId = c[0]
		}
		return types.NewDeleteOrderTx(p, ptr.OrderId, committeeId, ptr.Fee)
	})
}

func ConsensusInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseForm(); err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	summary, err := app.ConsensusSummary(parseUint64FromString(r.Form.Get("id")))
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set(ContentType, ApplicationJSON)
	w.WriteHeader(http.StatusOK)
	if _, e := w.Write(summary); e != nil {
		logger.Error(e.Error())
	}
}

func PeerInfo(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	peers, numInbound, numOutbound := app.P2P.GetAllInfos()
	write(w, &peerInfoResponse{
		ID:          app.P2P.ID(),
		NumPeers:    numInbound + numOutbound,
		NumInbound:  numInbound,
		NumOutbound: numOutbound,
		Peers:       peers,
	}, http.StatusOK)
}

func PeerBook(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	write(w, app.P2P.GetBookPeers(), http.StatusOK)
}

func Config(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	write(w, conf, http.StatusOK)
}

func FDCount(pid int32) (int, error) {
	cmd := []string{"-a", "-n", "-P", "-p", strconv.Itoa(int(pid))}
	out, err := Exec("lsof", cmd...)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(out), "\n")
	var ret []string
	for _, l := range lines[1:] {
		if len(l) == 0 {
			continue
		}
		ret = append(ret, l)
	}
	return len(ret), nil
}

func Exec(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return buf.Bytes(), err
	}

	if err := cmd.Wait(); err != nil {
		return buf.Bytes(), err
	}

	return buf.Bytes(), nil
}

func ResourceUsage(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	pm, err := mem.VirtualMemory() // os memory
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	c, err := cpu.Times(false) // os cpu
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	cp, err := cpu.Percent(0, false) // os cpu percent
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	d, err := disk.Usage("/") // os disk
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	name, err := p.Name()
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	cpuPercent, err := p.CPUPercent()
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	ioCounters, err := net.IOCounters(false)
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	status, err := p.Status()
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	fds, err := FDCount(p.Pid)
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	numThreads, err := p.NumThreads()
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	memPercent, err := p.MemoryPercent()
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	utc, err := p.CreateTime()
	if err != nil {
		write(w, err, http.StatusInternalServerError)
		return
	}
	write(w, resourceUsageResponse{
		Process: ProcessResourceUsage{
			Name:          name,
			Status:        status[0],
			CreateTime:    time.Unix(utc, 0).Format(time.RFC822),
			FDCount:       uint64(fds),
			ThreadCount:   uint64(numThreads),
			MemoryPercent: float64(memPercent),
			CPUPercent:    cpuPercent,
		},
		System: SystemResourceUsage{
			TotalRAM:        pm.Total,
			AvailableRAM:    pm.Available,
			UsedRAM:         pm.Used,
			UsedRAMPercent:  pm.UsedPercent,
			FreeRAM:         pm.Free,
			UsedCPUPercent:  cp[0],
			UserCPU:         c[0].User,
			SystemCPU:       c[0].System,
			IdleCPU:         c[0].Idle,
			TotalDisk:       d.Total,
			UsedDisk:        d.Used,
			UsedDiskPercent: d.UsedPercent,
			FreeDisk:        d.Free,
			ReceivedBytesIO: ioCounters[0].BytesRecv,
			WrittenBytesIO:  ioCounters[0].BytesSent,
		},
	}, http.StatusOK)
}

func txHandler(w http.ResponseWriter, r *http.Request, callback func(privateKey crypto.PrivateKeyI, ptr *txRequest) (lib.TransactionI, error)) {
	ptr := new(txRequest)
	if ok := unmarshal(w, r, ptr); !ok {
		return
	}
	keystore, ok := newKeystore(w)
	if !ok {
		return
	}
	privateKey, err := keystore.GetKey(ptr.Address, ptr.Password)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	state, ok := getStateMachineWithHeight(0, w)
	if !ok {
		return
	}
	if ptr.Fee == 0 {
		feeParams, e := state.GetParamsFee()
		if e != nil {
			write(w, e, http.StatusBadRequest)
			return
		}
		ptr.Fee = feeParams.MessageSendFee
	}
	p, err := callback(privateKey, ptr)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	if ptr.Submit {
		submitTx(w, p)
	} else {
		bz, e := lib.MarshalJSONIndent(p)
		if e != nil {
			write(w, e, http.StatusBadRequest)
			return
		}
		if _, err = w.Write(bz); err != nil {
			logger.Error(err.Error())
			return
		}
	}
}

func keystoreHandler(w http.ResponseWriter, r *http.Request, callback func(keystore *crypto.Keystore, ptr *keystoreRequest) (any, error)) {
	keystore, ok := newKeystore(w)
	if !ok {
		return
	}
	ptr := new(keystoreRequest)
	if ok = unmarshal(w, r, ptr); !ok {
		return
	}
	p, err := callback(keystore, ptr)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func heightAndAddressParams(w http.ResponseWriter, r *http.Request, callback func(*fsm.StateMachine, lib.HexBytes) (any, lib.ErrorI)) {
	req := new(heightAndAddressRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	if req.Address == nil {
		write(w, types.ErrAddressEmpty(), http.StatusBadRequest)
		return
	}
	p, err := callback(state, req.Address)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func heightAndIdParams(w http.ResponseWriter, r *http.Request, callback func(*fsm.StateMachine, uint64) (any, lib.ErrorI)) {
	req := new(heightAndIdRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state, req.ID)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func orderParams(w http.ResponseWriter, r *http.Request, callback func(s *fsm.StateMachine, request *orderRequest) (any, lib.ErrorI)) {
	req := new(orderRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state, req)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func heightParams(w http.ResponseWriter, r *http.Request, callback func(s *fsm.StateMachine) (any, lib.ErrorI)) {
	req := new(heightRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func heightPaginated(w http.ResponseWriter, r *http.Request, callback func(s *fsm.StateMachine, p *paginatedHeightRequest) (any, lib.ErrorI)) {
	req := new(paginatedHeightRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state, req)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func heightIndexer(w http.ResponseWriter, r *http.Request, callback func(s lib.StoreI, h uint64, p lib.PageParams) (any, lib.ErrorI)) {
	req := new(paginatedHeightRequest)
	if ok := unmarshal(w, r, req); !ok {
		return
	}
	s, ok := setupStore(w)
	if !ok {
		return
	}
	defer s.Discard()
	if req.Height == 0 {
		req.Height = s.Version() - 1
	}
	p, err := callback(s, req.Height, req.PageParams)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func hashIndexer(w http.ResponseWriter, r *http.Request, callback func(s lib.StoreI, h lib.HexBytes) (any, lib.ErrorI)) {
	req := new(hashRequest)
	if ok := unmarshal(w, r, req); !ok {
		return
	}
	s, ok := setupStore(w)
	if !ok {
		return
	}
	defer s.Discard()
	bz, err := lib.StringToBytes(req.Hash)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	p, err := callback(s, bz)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func addrIndexer(w http.ResponseWriter, r *http.Request, callback func(s lib.StoreI, a crypto.AddressI, p lib.PageParams) (any, lib.ErrorI)) {
	req := new(paginatedAddressRequest)
	if ok := unmarshal(w, r, req); !ok {
		return
	}
	s, ok := setupStore(w)
	if !ok {
		return
	}
	defer s.Discard()
	if req.Address == nil {
		write(w, types.ErrAddressEmpty(), http.StatusBadRequest)
		return
	}
	p, err := callback(s, crypto.NewAddressFromBytes(req.Address), req.PageParams)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

type hashRequest struct {
	Hash string `json:"hash"`
}

type addressRequest struct {
	Address lib.HexBytes `json:"address"`
}

type committeesRequest struct {
	committees string
}

func StringToCommittees(s string) (committees []uint64, err error) {
	i, err := strconv.ParseUint(s, 10, 64) // single int is an option for subsidy txn
	if err == nil {
		return []uint64{i}, nil
	}
	commaSeparatedArr := strings.Split(strings.ReplaceAll(s, " ", ""), ",")
	if len(commaSeparatedArr) == 0 {
		return nil, ErrStringToCommittee(s)
	}
	for _, c := range commaSeparatedArr {
		ui, e := strconv.ParseUint(c, 10, 64)
		if e != nil {
			return nil, e
		}
		committees = append(committees, ui)
	}
	return
}

type heightRequest struct {
	Height uint64 `json:"height"`
}

type orderRequest struct {
	CommitteeId uint64 `json:"committeeId"`
	OrderId     uint64 `json:"orderId"`
	heightRequest
}

type heightsRequest struct {
	heightRequest
	StartHeight uint64 `json:"startHeight"`
}

type idRequest struct {
	ID uint64 `json:"id"`
}
type passwordRequest struct {
	Password string `json:"password"`
}
type voteRequest struct {
	Approve  bool            `json:"approve"`
	Proposal json.RawMessage `json:"proposal"`
}

type paginatedAddressRequest struct {
	addressRequest
	lib.PageParams
}

type paginatedHeightRequest struct {
	heightRequest
	lib.PageParams
	lib.ValidatorFilters
}

type heightAndAddressRequest struct {
	heightRequest
	addressRequest
}

type heightAndIdRequest struct {
	heightRequest
	idRequest
}

type keystoreRequest struct {
	addressRequest
	passwordRequest
	PrivateKey lib.HexBytes `json:"privateKey"`
	crypto.EncryptedPrivateKey
}

type resourceUsageResponse struct {
	Process ProcessResourceUsage `json:"process"`
	System  SystemResourceUsage  `json:"system"`
}

type peerInfoResponse struct {
	ID          *lib.PeerAddress `json:"id"`
	NumPeers    int              `json:"numPeers"`
	NumInbound  int              `json:"numInbound"`
	NumOutbound int              `json:"numOutbound"`
	Peers       []*lib.PeerInfo  `json:"peers"`
}

type ProcessResourceUsage struct {
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	CreateTime    string  `json:"createTime"`
	FDCount       uint64  `json:"fdCount"`
	ThreadCount   uint64  `json:"threadCount"`
	MemoryPercent float64 `json:"usedMemoryPercent"`
	CPUPercent    float64 `json:"usedCPUPercent"`
}

type SystemResourceUsage struct {
	// ram
	TotalRAM       uint64  `json:"totalRAM"`
	AvailableRAM   uint64  `json:"availableRAM"`
	UsedRAM        uint64  `json:"usedRAM"`
	UsedRAMPercent float64 `json:"usedRAMPercent"`
	FreeRAM        uint64  `json:"freeRAM"`
	// CPU
	UsedCPUPercent float64 `json:"usedCPUPercent"`
	UserCPU        float64 `json:"userCPU"`
	SystemCPU      float64 `json:"systemCPU"`
	IdleCPU        float64 `json:"idleCPU"`
	// disk
	TotalDisk       uint64  `json:"totalDisk"`
	UsedDisk        uint64  `json:"usedDisk"`
	UsedDiskPercent float64 `json:"usedDiskPercent"`
	FreeDisk        uint64  `json:"freeDisk"`
	// io
	ReceivedBytesIO uint64 `json:"ReceivedBytesIO"`
	WrittenBytesIO  uint64 `json:"WrittenBytesIO"`
}

func (h *heightRequest) GetHeight() uint64 {
	return h.Height
}

type queryWithHeight interface {
	GetHeight() uint64
}

func newKeystore(w http.ResponseWriter) (k *crypto.Keystore, ok bool) {
	k, err := crypto.NewKeystoreFromFile(conf.DataDirPath)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	ok = true
	return
}

func submitTx(w http.ResponseWriter, tx any) (ok bool) {
	bz, err := lib.Marshal(tx)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	if err = app.SendTxMsg(lib.CanopyCommitteeId, bz); err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, crypto.HashString(bz), http.StatusOK)
	return true
}

func getStateMachineFromHeightParams(w http.ResponseWriter, r *http.Request, ptr queryWithHeight) (sm *fsm.StateMachine, ok bool) {
	if ok = unmarshal(w, r, ptr); !ok {
		return
	}
	return getStateMachineWithHeight(ptr.GetHeight(), w)
}

func parseUint64FromString(s string) uint64 {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return uint64(i)
}

func getDoubleStateMachineFromHeightParams(w http.ResponseWriter, r *http.Request, p httprouter.Params) (sm1, sm2 *fsm.StateMachine, o *jsondiff.Options, ok bool) {
	request, opts := new(heightsRequest), jsondiff.Options{}
	switch r.Method {
	case http.MethodGet:
		opts = jsondiff.DefaultHTMLOptions()
		opts.ChangedSeparator = " <- "
		if err := r.ParseForm(); err != nil {
			ok = false
			write(w, err, http.StatusBadRequest)
			return
		}
		request.Height = parseUint64FromString(r.Form.Get("height"))
		request.StartHeight = parseUint64FromString(r.Form.Get("startHeight"))
	case http.MethodPost:
		opts = jsondiff.DefaultConsoleOptions()
		if ok = unmarshal(w, r, request); !ok {
			return
		}
	}
	sm1, ok = getStateMachineWithHeight(request.Height, w)
	if !ok {
		return
	}
	if request.StartHeight == 0 {
		request.StartHeight = sm1.Height() - 1
	}
	sm2, ok = getStateMachineWithHeight(request.StartHeight, w)
	o = &opts
	return
}

func getStateMachineWithHeight(height uint64, w http.ResponseWriter) (sm *fsm.StateMachine, ok bool) {
	s, ok := setupStore(w)
	if !ok {
		return
	}
	return setupStateMachine(height, s, w)
}

func setupStore(w http.ResponseWriter) (lib.StoreI, bool) {
	s, err := store.NewStoreWithDB(db, logger)
	if err != nil {
		write(w, ErrNewStore(err), http.StatusInternalServerError)
		return nil, false
	}
	return s, true
}

// TODO likely a memory leak here from un-discarded stores
func setupStateMachine(height uint64, s lib.StoreI, w http.ResponseWriter) (*fsm.StateMachine, bool) {
	state, err := fsm.New(conf, s, logger)
	if err != nil {
		write(w, ErrNewFSM(err), http.StatusInternalServerError)
		return nil, false
	}
	if height != 0 {
		state, err = state.TimeMachine(height)
		if err != nil {
			write(w, ErrTimeMachine(err), http.StatusInternalServerError)
		}
	}
	return state, true
}

func unmarshal(w http.ResponseWriter, r *http.Request, ptr interface{}) bool {
	bz, err := io.ReadAll(io.LimitReader(r.Body, int64(units.MB)))
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return false
	}
	defer func() { _ = r.Body.Close() }()
	if err = json.Unmarshal(bz, ptr); err != nil {
		write(w, err, http.StatusBadRequest)
		return false
	}
	return true
}

func write(w http.ResponseWriter, payload interface{}, code int) {
	w.Header().Set(ContentType, ApplicationJSON)
	w.WriteHeader(code)
	bz, _ := json.MarshalIndent(payload, "", "  ")
	if _, err := w.Write(bz); err != nil {
		logger.Error(err.Error())
	}
}

type routes map[string]struct {
	Method      string
	Path        string
	HandlerFunc httprouter.Handle
	AdminOnly   bool
}

func (r routes) New() (router *httprouter.Router) {
	router = httprouter.New()
	for _, route := range r {
		if !route.AdminOnly {
			router.Handle(route.Method, route.Path, route.HandlerFunc)
		}
	}
	return
}

func (r routes) NewAdmin() (router *httprouter.Router) {
	router = httprouter.New()
	for _, route := range r {
		if route.AdminOnly {
			router.Handle(route.Method, route.Path, route.HandlerFunc)
		}
	}
	return
}

func debugHandler(routeName string) httprouter.Handle {
	f := func(w http.ResponseWriter, r *http.Request) {}
	switch routeName {
	case DebugHeapRouteName, DebugRoutineRouteName, DebugBlockedRouteName:
		f = func(w http.ResponseWriter, r *http.Request) {
			pprof.Handler(routeName).ServeHTTP(w, r)
		}
	case DebugCPURouteName:
		f = pprof.Profile
	}
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		f(w, r)
	}
}

func runStaticFileServer(fileSys fs.FS, dir, port string) {
	distFS, err := fs.Sub(fileSys, dir)
	if err != nil {
		logger.Error(fmt.Sprintf("an error occurred running the static file server for %s: %s", dir, err.Error()))
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(distFS)))
	go func() {
		logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), mux).Error())
	}()
}

func logsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		filePath := filepath.Join(conf.DataDirPath, lib.LogDirectory, lib.LogFileName)
		f, _ := os.ReadFile(filePath)
		split := bytes.Split(f, []byte("\n"))
		var flipped []byte
		for i := len(split) - 1; i >= 0; i-- {
			flipped = append(append(flipped, split[i]...), []byte("\n")...)
		}
		if _, err := w.Write(flipped); err != nil {
			logger.Error(err.Error())
		}
	}
}
