package rpc

import (
	"encoding/json"
	"github.com/alecthomas/units"
	"github.com/dgraph-io/badger/v4"
	"github.com/ginchuco/ginchu/consensus"
	"github.com/ginchuco/ginchu/fsm"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/store"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"io"
	"net/http"
	"net/http/pprof"
	"time"
)

const (
	ContentType     = "Content-Type"
	ApplicationJSON = "application/json; charset=utf-8"
	version         = "0.0.0-alpha"
	colon           = ":"

	VersionRouteName        = "version"
	TxRouteName             = "tx"
	HeightRouteName         = "height"
	AccountRouteName        = "account"
	AccountsRouteName       = "accounts"
	PoolRouteName           = "pool"
	PoolsRouteName          = "pools"
	ValidatorRouteName      = "validator"
	ValidatorsRouteName     = "validators"
	ConsValidatorsRouteName = "cons-validators"
	NonSignersRouteName     = "non-signers"
	SupplyRouteName         = "supply"
	ParamRouteName          = "params"
	FeeParamRouteName       = "fee-params"
	GovParamRouteName       = "gov-params"
	ConParamsRouteName      = "con-params"
	ValParamRouteName       = "val-params"
	StateRouteName          = "state"
	CertByHeightRouteName   = "cert-by-height"
	BlocksRouteName         = "blocks"
	BlockByHeightRouteName  = "block-by-height"
	BlockByHashRouteName    = "block-by-hash"
	TxsByHeightRouteName    = "txs-by-height"
	TxsBySenderRouteName    = "txs-by-sender"
	TxsByRecRouteName       = "txs-by-rec"
	TxByHashRouteName       = "tx-by-hash"
	PendingRouteName        = "pending"
	// debug
	DebugBlockedRouteName = "blocked"
	DebugHeapRouteName    = "heap"
	DebugCPURouteName     = "cpu"
	DebugRoutineRouteName = "routine"
)

var (
	app    *consensus.Consensus
	db     *badger.DB
	conf   lib.Config
	logger lib.LoggerI

	router = routes{
		VersionRouteName:        {Method: http.MethodGet, Path: "/v1/", HandlerFunc: Version},
		TxRouteName:             {Method: http.MethodPost, Path: "/v1/tx", HandlerFunc: Transaction},
		HeightRouteName:         {Method: http.MethodPost, Path: "/v1/query/height", HandlerFunc: Height},
		AccountRouteName:        {Method: http.MethodPost, Path: "/v1/query/account", HandlerFunc: Account},
		AccountsRouteName:       {Method: http.MethodPost, Path: "/v1/query/accounts", HandlerFunc: Accounts},
		PoolRouteName:           {Method: http.MethodPost, Path: "/v1/query/pool", HandlerFunc: Pool},
		PoolsRouteName:          {Method: http.MethodPost, Path: "/v1/query/pools", HandlerFunc: Pools},
		ValidatorRouteName:      {Method: http.MethodPost, Path: "/v1/query/validator", HandlerFunc: Validator},
		ValidatorsRouteName:     {Method: http.MethodPost, Path: "/v1/query/validators", HandlerFunc: Validators},
		ConsValidatorsRouteName: {Method: http.MethodPost, Path: "/v1/query/cons-validators", HandlerFunc: ConsValidators},
		NonSignersRouteName:     {Method: http.MethodPost, Path: "/v1/query/non-signers", HandlerFunc: NonSigners},
		ParamRouteName:          {Method: http.MethodPost, Path: "/v1/query/params", HandlerFunc: Params},
		SupplyRouteName:         {Method: http.MethodPost, Path: "/v1/query/supply", HandlerFunc: Supply},
		FeeParamRouteName:       {Method: http.MethodPost, Path: "/v1/query/fee-params", HandlerFunc: FeeParams},
		GovParamRouteName:       {Method: http.MethodPost, Path: "/v1/query/gov-params", HandlerFunc: GovParams},
		ConParamsRouteName:      {Method: http.MethodPost, Path: "/v1/query/con-params", HandlerFunc: ConParams},
		ValParamRouteName:       {Method: http.MethodPost, Path: "/v1/query/val-params", HandlerFunc: ValParams},
		StateRouteName:          {Method: http.MethodPost, Path: "/v1/query/state", HandlerFunc: State},
		CertByHeightRouteName:   {Method: http.MethodPost, Path: "/v1/query/cert-by-height", HandlerFunc: CertByHeight},
		BlockByHeightRouteName:  {Method: http.MethodPost, Path: "/v1/query/block-by-height", HandlerFunc: BlockByHeight},
		BlocksRouteName:         {Method: http.MethodPost, Path: "/v1/query/blocks", HandlerFunc: Blocks},
		BlockByHashRouteName:    {Method: http.MethodPost, Path: "/v1/query/block-by-hash", HandlerFunc: BlockByHash},
		TxsByHeightRouteName:    {Method: http.MethodPost, Path: "/v1/query/txs-by-height", HandlerFunc: TransactionsByHeight},
		TxsBySenderRouteName:    {Method: http.MethodPost, Path: "/v1/query/txs-by-sender", HandlerFunc: TransactionsBySender},
		TxsByRecRouteName:       {Method: http.MethodPost, Path: "/v1/query/txs-by-rec", HandlerFunc: TransactionsByRecipient},
		TxByHashRouteName:       {Method: http.MethodPost, Path: "/v1/query/tx-by-hash", HandlerFunc: TransactionByHash},
		PendingRouteName:        {Method: http.MethodPost, Path: "/v1/query/pending", HandlerFunc: Pending},
		// debug
		DebugBlockedRouteName: {Method: http.MethodPost, Path: "/debug/blocked", HandlerFunc: debugHandler(DebugBlockedRouteName)},
		DebugHeapRouteName:    {Method: http.MethodPost, Path: "/debug/heap", HandlerFunc: debugHandler(DebugHeapRouteName)},
		DebugCPURouteName:     {Method: http.MethodPost, Path: "/debug/cpu", HandlerFunc: debugHandler(DebugHeapRouteName)},
		DebugRoutineRouteName: {Method: http.MethodPost, Path: "/debug/routine", HandlerFunc: debugHandler(DebugRoutineRouteName)},
	}
)

func StartRPC(a *consensus.Consensus, c lib.Config, l lib.LoggerI) {
	cor := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "OPTIONS", "POST"},
	})
	srv := &http.Server{
		Addr:    colon + c.Port,
		Handler: cor.Handler(http.TimeoutHandler(router.New(), time.Duration(c.TimeoutS)*time.Second, ErrServerTimeout().Error())),
	}
	s := a.FSM.Store().(lib.StoreI)
	app, conf, db, logger = a, c, s.DB(), l
	l.Infof("Starting RPC server at port :%s", c.Port)
	go func() { l.Fatal(srv.ListenAndServe().Error()) }()
}

func Version(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	write(w, version, http.StatusOK)
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
	heightPaginated(w, r, func(s *fsm.StateMachine, p lib.PageParams) (interface{}, lib.ErrorI) {
		return s.GetAccountsPaginated(p)
	})
}

func Pool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightAndNameParams(w, r, func(s *fsm.StateMachine, n string) (interface{}, lib.ErrorI) {
		return s.GetPool(types.PoolID(types.PoolID_value[n]))
	})
}

func Pools(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p lib.PageParams) (interface{}, lib.ErrorI) {
		return s.GetPoolsPaginated(p)
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
	heightPaginated(w, r, func(s *fsm.StateMachine, p lib.PageParams) (interface{}, lib.ErrorI) {
		return s.GetValidatorsPaginated(p)
	})
}

func ConsValidators(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	heightPaginated(w, r, func(s *fsm.StateMachine, p lib.PageParams) (interface{}, lib.ErrorI) {
		return s.GetConsValidatorsPaginated(p)
	})
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

func Transaction(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tx := new(lib.Transaction)
	if ok := unmarshal(w, r, tx); !ok {
		return
	}
	bz, err := lib.Marshal(tx)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	if err = app.NewTx(bz); err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, crypto.HashString(bz), http.StatusOK)
}

func heightAndAddressParams(w http.ResponseWriter, r *http.Request, callback func(*fsm.StateMachine, lib.HexBytes) (any, lib.ErrorI)) {
	req := new(heightAndAddressRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state, req.Address)
	if err != nil {
		write(w, err, http.StatusBadRequest)
		return
	}
	write(w, p, http.StatusOK)
}

func heightAndNameParams(w http.ResponseWriter, r *http.Request, callback func(*fsm.StateMachine, string) (any, lib.ErrorI)) {
	req := new(heightAndNameRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state, req.Name)
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

func heightPaginated(w http.ResponseWriter, r *http.Request, callback func(s *fsm.StateMachine, p lib.PageParams) (any, lib.ErrorI)) {
	req := new(paginatedHeightRequest)
	state, ok := getStateMachineFromHeightParams(w, r, req)
	if !ok {
		return
	}
	p, err := callback(state, req.PageParams)
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

type heightRequest struct {
	Height uint64 `json:"height"`
}

type nameRequest struct {
	Name string `json:"name"`
}

type paginatedAddressRequest struct {
	addressRequest
	lib.PageParams
}

type paginatedHeightRequest struct {
	heightRequest
	lib.PageParams
}

type heightAndAddressRequest struct {
	heightRequest
	addressRequest
}

type heightAndNameRequest struct {
	heightRequest
	nameRequest
}

func (h *heightRequest) GetHeight() uint64 {
	return h.Height
}

type queryWithHeight interface {
	GetHeight() uint64
}

func getStateMachineFromHeightParams(w http.ResponseWriter, r *http.Request, ptr queryWithHeight) (sm *fsm.StateMachine, ok bool) {
	if ok = unmarshal(w, r, ptr); !ok {
		return
	}
	return getStateMachineWithHeight(ptr.GetHeight(), w)
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
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Error(err.Error())
	}
}

type routes map[string]struct {
	Method      string
	Path        string
	HandlerFunc httprouter.Handle
}

func (r routes) New() (router *httprouter.Router) {
	router = httprouter.New()
	for _, route := range r {
		router.Handle(route.Method, route.Path, route.HandlerFunc)
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
