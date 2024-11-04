package main

import (
	"github.com/canopy-network/canopy/lib"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "query the blockchain rpc",
}

var (
	height, startHeight, pageNumber, perPage, unstaking, delegated, paused = uint64(0), uint64(0), 0, 0, "", "", ""
)

func init() {
	queryCmd.PersistentFlags().Uint64Var(&height, "height", 0, "historical height for the query, 0 is latest")
	queryCmd.PersistentFlags().Uint64Var(&height, "start-height", 0, "starting height for queries with a range")
	queryCmd.PersistentFlags().IntVar(&pageNumber, "page-number", 0, "page number on a paginated call")
	queryCmd.PersistentFlags().IntVar(&perPage, "per-page", 0, "number of items per page on a paginated call")
	queryCmd.PersistentFlags().StringVar(&unstaking, "unstaking", "", "yes = only unstaking validators, no = only non-unstaking validators")
	queryCmd.PersistentFlags().StringVar(&paused, "paused", "", "yes = only paused validators, no = only unpaused validators")
	queryCmd.PersistentFlags().StringVar(&delegated, "delegated", "", "yes = only delegated validators, no = only non-delegated validators")
	queryCmd.AddCommand(heightCmd)
	queryCmd.AddCommand(accountCmd)
	queryCmd.AddCommand(accountsCmd)
	queryCmd.AddCommand(poolCmd)
	queryCmd.AddCommand(poolsCmd)
	queryCmd.AddCommand(validatorCmd)
	queryCmd.AddCommand(validatorsCmd)
	queryCmd.AddCommand(consValidatorsCmd)
	queryCmd.AddCommand(nonSignersCmd)
	queryCmd.AddCommand(paramsCmd)
	queryCmd.AddCommand(supplyCmd)
	queryCmd.AddCommand(stateCmd)
	queryCmd.AddCommand(stateDiffCmd)
	queryCmd.AddCommand(certCmd)
	queryCmd.AddCommand(blkByHeightCmd)
	queryCmd.AddCommand(blkByHashCmd)
	queryCmd.AddCommand(blocksCmd)
	queryCmd.AddCommand(txsByHeightCmd)
	queryCmd.AddCommand(txsBySenderCmd)
	queryCmd.AddCommand(txsByRecCmd)
	queryCmd.AddCommand(txByHashCmd)
	queryCmd.AddCommand(pendingTxsCmd)
	queryCmd.AddCommand(proposalsCmd)
	queryCmd.AddCommand(pollCmd)
}

var (
	heightCmd = &cobra.Command{
		Use:   "height",
		Short: "query the height of the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Height())
		},
	}

	accountCmd = &cobra.Command{
		Use:   "account <address> --height=1",
		Short: "query an account on the blockchain",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Account(height, args[0]))
		},
	}

	accountsCmd = &cobra.Command{
		Use:   "accounts --height=1 --per-page=10 --page-number=1",
		Short: "query all accounts on the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Accounts(getPaginatedArgs()))
		},
	}

	poolCmd = &cobra.Command{
		Use:   "pool <committee_id> --height=1",
		Short: "query a pool on the blockchain",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Pool(getPoolArgs(args)))
		},
	}

	poolsCmd = &cobra.Command{
		Use:   "pools --height=1 --per-page=10 --page-number=1",
		Short: "query all pools on the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Pools(getPaginatedArgs()))
		},
	}

	validatorCmd = &cobra.Command{
		Use:   "validator <address> --height=1",
		Short: "query a validator on the blockchain",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Validator(height, args[0]))
		},
	}

	validatorsCmd = &cobra.Command{
		Use:   "validators --height=1 --per-page=10 --page-number=1 --unstaking=yes --paused=no",
		Short: "query all validators on the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Validators(getFilterArgs()))
		},
	}

	consValidatorsCmd = &cobra.Command{
		Use:   "cons-validators --height=1 --per-page=10 --page-number=1",
		Short: "query all consensus participating validators on the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.ConsValidators(getPaginatedArgs()))
		},
	}

	nonSignersCmd = &cobra.Command{
		Use:   "non-signers --height=1",
		Short: "query all bft non signing validators and their non-sign counter",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.NonSigners(height))
		},
	}

	paramsCmd = &cobra.Command{
		Use:   "params --height=1",
		Short: "query all governance params",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Params(height))
		},
	}

	supplyCmd = &cobra.Command{
		Use:   "supply --height=1",
		Short: "query the blockchain token supply",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Supply(height))
		},
	}

	stateCmd = &cobra.Command{
		Use:   "state --height=1",
		Short: "query the blockchain world state",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.State(height))
		},
	}

	stateDiffCmd = &cobra.Command{
		Use:   "state-diff --start-height=1 --height=2",
		Short: "query the blockchain state difference between two heights",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.StateDiff(startHeight, height))
		},
	}

	certCmd = &cobra.Command{
		Use:   "certificate <height>",
		Short: "query a quorum certificate for a height",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.CertByHeight(uint64(argToInt(args[0]))))
		},
	}

	blkByHeightCmd = &cobra.Command{
		Use:   "block-by-height <height> ",
		Short: "query a block at a height",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.CertByHeight(uint64(argToInt(args[0]))))
		},
	}

	blkByHashCmd = &cobra.Command{
		Use:   "block-by-hash <hash>",
		Short: "query a block with a hash",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.BlockByHash(args[0]))
		},
	}

	blocksCmd = &cobra.Command{
		Use:   "blocks --per-page=10 --page-number=1",
		Short: "query blocks from the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			_, p := getPaginatedArgs()
			writeToConsole(client.Blocks(p))
		},
	}

	txsByHeightCmd = &cobra.Command{
		Use:   "txs-by-height <height> --per-page=10 --page-number=1",
		Short: "query txs at a certain height",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			height = uint64(argToInt(args[0]))
			writeToConsole(client.TransactionsByHeight(getPaginatedArgs()))
		},
	}

	txsBySenderCmd = &cobra.Command{
		Use:   "txs-by-sender <address> --per-page=10 --page-number=1",
		Short: "query txs from a sender address",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			_, p := getPaginatedArgs()
			writeToConsole(client.TransactionsBySender(args[0], p))
		},
	}

	txsByRecCmd = &cobra.Command{
		Use:   "txs-by-rec <address> --per-page=10 --page-number=1",
		Short: "query txs from a recipient address",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			_, p := getPaginatedArgs()
			writeToConsole(client.TransactionsByRecipient(args[0], p))
		},
	}

	txByHashCmd = &cobra.Command{
		Use:   "tx-by-hash <hash>",
		Short: "query a transaction by its hash",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.TransactionByHash(args[0]))
		},
	}

	pendingTxsCmd = &cobra.Command{
		Use:   "pending-txs --per-page=10 --page-number=1",
		Short: "query a transactions in the local mempool but not yet included in a block",
		Run: func(cmd *cobra.Command, args []string) {
			_, p := getPaginatedArgs()
			writeToConsole(client.Pending(p))
		},
	}

	proposalsCmd = &cobra.Command{
		Use:   "proposals",
		Short: "query the nodes votes on governance proposals",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Proposals())
		},
	}

	pollCmd = &cobra.Command{
		Use:   "poll",
		Short: "query the nodes polling results on governance proposals",
		Run: func(cmd *cobra.Command, args []string) {
			writeToConsole(client.Poll())
		},
	}
)

func getPoolArgs(args []string) (h uint64, id uint64) {
	h = height
	id = uint64(argToInt(args[0]))
	return
}

func getPaginatedArgs() (h uint64, params lib.PageParams) {
	h = height
	params = lib.PageParams{
		PageNumber: pageNumber,
		PerPage:    perPage,
	}
	return
}

func getFilterArgs() (h uint64, params lib.PageParams, filters lib.ValidatorFilters) {
	h, params = getPaginatedArgs()
	switch {
	case strings.Contains(strings.ToLower(unstaking), "y"):
		filters.Unstaking = lib.FilterOption_MustBe
	case strings.Contains(strings.ToLower(unstaking), "n"):
		filters.Unstaking = lib.FilterOption_Exclude
	}
	switch {
	case strings.Contains(strings.ToLower(paused), "y"):
		filters.Paused = lib.FilterOption_MustBe
	case strings.Contains(strings.ToLower(paused), "n"):
		filters.Paused = lib.FilterOption_Exclude
	}
	switch {
	case strings.Contains(strings.ToLower(delegated), "y"):
		filters.Delegate = lib.FilterOption_MustBe
	case strings.Contains(strings.ToLower(delegated), "n"):
		filters.Delegate = lib.FilterOption_Exclude
	}
	return
}

func argToInt(arg string) int {
	i, err := strconv.Atoi(arg)
	if err != nil {
		l.Fatal(err.Error())
	}
	return i
}
