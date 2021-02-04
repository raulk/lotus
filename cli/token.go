package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	init3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/init"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors/builtin/token"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/lib/tablewriter"

	"github.com/urfave/cli/v2"
)

var tokenCmd = &cli.Command{
	Name:  "token",
	Usage: "Interact with token actors",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "confidence",
			Usage: "number of block confirmations to wait for",
			Value: int(build.MessageConfidence),
		},
	},
	Subcommands: []*cli.Command{
		tokenCreateCmd,
		tokenInfoCmd,
		tokenBalanceCmd,
		tokenHoldersCmd,
		tokenDelegationsCmd,
		tokenTransferCmd,
		tokenApproveCmd,
	},
}

var tokenCreateCmd = &cli.Command{
	Name:  "create",
	Usage: "Create a new token actor",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "owner",
			Usage: "the wallet address to create the token from; it will own the total supply; will use the wallet default address if not provided",
		},
		&cli.StringFlag{
			Name:     "name",
			Usage:    "token name",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "symbol",
			Usage:    "token symbol",
			Required: true,
		},
		&cli.Uint64Flag{
			Name:  "decimals",
			Usage: "token decimals; by default, 18 (divisible up to atto)",
			Value: 18, // atto
		},
		&cli.StringFlag{
			Name:     "supply",
			Usage:    "total supply",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "icon",
			Usage: "base64-encoded svg icon",
		},
	},
	Action: func(cctx *cli.Context) error {
		w := cctx.App.Writer

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		var (
			confidence = uint64(cctx.Int("confidence"))

			name     = cctx.String("name")
			symbol   = cctx.String("symbol")
			iconb64  = cctx.String("icon")
			decimals = cctx.Uint64("decimals")

			owner  address.Address
			supply abi.TokenAmount
			icon   []byte
		)

		if c := cctx.String("owner"); c == "" {
			owner, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			owner, err = address.NewFromString(c)
			if err != nil {
				return fmt.Errorf("failed to parse owner address: %w", err)
			}
		}

		if supply, err = types.BigFromString(cctx.String("supply")); err != nil {
			return fmt.Errorf("failed to parse supply: %w", err)
		}

		if icon, err = base64.StdEncoding.DecodeString(iconb64); err != nil {
			return fmt.Errorf("failed to decode base64 icon: %w", err)
		}

		_, _ = fmt.Fprintf(w, "creating token %s (%s) with owner %s and total supply %d\n", name, symbol, owner, supply)

		mcid, err := api.TokenCreate(ctx, owner, &token.Info{
			Name:        name,
			Symbol:      symbol,
			Decimals:    decimals,
			Icon:        icon,
			TotalSupply: supply,
		})
		if err != nil {
			return fmt.Errorf("token creation failed: %w", err)
		}

		_, _ = fmt.Fprintf(w, "message CID: %s\n", mcid)

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		if err = processResult(w, result); err != nil {
			return err
		}

		// get address of newly created miner
		var ret init3.ExecReturn
		if err := ret.UnmarshalCBOR(bytes.NewReader(result.Receipt.Return)); err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cctx.App.Writer, "created new token actor: ", ret.IDAddress, ret.RobustAddress)
		return nil
	},
}

var tokenInfoCmd = &cli.Command{
	Name:  "info",
	Usage: "Retrieve the basic info of a token actor",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		t := cctx.String("token")
		addr, err := address.NewFromString(t)
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", t, err)
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		info, err := api.TokenInfo(ctx, addr)
		if err != nil {
			return fmt.Errorf("failed to retrieve token info: %w", err)
		}

		output, err := json.Marshal(info)
		if err != nil {
			return fmt.Errorf("failed to serialize token info into JSON: %w", err)
		}

		_, _ = fmt.Fprintln(cctx.App.Writer, string(output))
		return nil
	},
}

var tokenBalanceCmd = &cli.Command{
	Name:      "balance",
	Usage:     "Retrieve the balances of token holders",
	ArgsUsage: "<holders...>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) (err error) {
		if cctx.NArg() < 1 {
			return ShowHelp(cctx, fmt.Errorf("must specify at least one holder address"))
		}

		t := cctx.String("token")
		token, err := address.NewFromString(t)
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", t, err)
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		holders := make([]address.Address, 0, cctx.NArg())
		for _, a := range cctx.Args().Slice() {
			addr, err := address.NewFromString(a)
			if err != nil {
				return fmt.Errorf("failed to parse address %s: %w", a, err)
			}
			holders = append(holders, addr)
		}

		for _, holder := range holders {
			balance, err := api.TokenBalanceOf(ctx, token, holder)
			if err != nil {
				_, _ = fmt.Fprintln(cctx.App.Writer, holder, err)
				continue
			}
			_, _ = fmt.Fprintln(cctx.App.Writer, holder, balance)
		}
		return nil
	},
}

var tokenHoldersCmd = &cli.Command{
	Name:  "holders",
	Usage: "Retrieve all token holders and their balances",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) (err error) {
		w := cctx.App.Writer

		t := cctx.String("token")
		token, err := address.NewFromString(t)
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", t, err)
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		holders, err := api.TokenGetHolders(ctx, token)
		if err != nil {
			return fmt.Errorf("failed to get holders: %w", err)
		}

		tw := tablewriter.New(tablewriter.Col("ID"),
			tablewriter.Col("Robust"),
			tablewriter.Col("Balance"))

		for _, h := range holders {
			tw.Write(map[string]interface{}{
				"ID":      h.IDAddress,
				"PubKey":  h.PubKeyAddress,
				"Balance": h.Balance,
			})
		}

		_ = tw.Flush(w)

		return nil
	},
}

var tokenDelegationsCmd = &cli.Command{
	Name:      "delegations",
	Usage:     "Retrieve all token spending delegations from the provided holder address for the provided token",
	ArgsUsage: "<holder>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return ShowHelp(cctx, fmt.Errorf("must specify the holder's address"))
		}

		t := cctx.String("token")
		token, err := address.NewFromString(t)
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", t, err)
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		holderAddr, err := address.NewFromString(cctx.Args().Get(1))
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", holderAddr, err)
		}

		delegations, err := api.TokenGetSpendersOf(ctx, token, holderAddr)
		if err != nil {
			return fmt.Errorf("failed to get holders: %w", err)
		}

		for spender, balance := range delegations {
			_, _ = fmt.Fprintln(cctx.App.Writer, spender, balance)
		}

		return nil
	},
}

var tokenTransferCmd = &cli.Command{
	Name:      "transfer",
	Usage:     "Transfer a token balance, either directly or via a delegation",
	ArgsUsage: "<recipient> <amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "delegated-from",
			Usage: "if not empty, this will be treated as a delegated transfer from this address",
		},
		&cli.StringFlag{
			Name:  "sender",
			Usage: "sender address and signer; will use the wallet default address if not provided",
		},
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		w := cctx.App.Writer

		if cctx.NArg() != 2 {
			return ShowHelp(cctx, fmt.Errorf("must specify recipient and amount"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		var (
			confidence = uint64(cctx.Int("confidence"))

			sender    address.Address
			recipient address.Address
			token     address.Address
			amount    abi.TokenAmount
		)

		// sender address; default to wallet default address.
		if c := cctx.String("sender"); c == "" {
			sender, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			sender, err = address.NewFromString(c)
			if err != nil {
				return fmt.Errorf("failed to parse creator address: %w", err)
			}
		}

		// token address.
		token, err = address.NewFromString(cctx.String("token"))
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", token, err)
		}

		// recipient address.
		recipient, err = address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		// amount.
		if amount, err = types.BigFromString(cctx.Args().Get(1)); err != nil {
			return fmt.Errorf("failed to parse amount: %w", err)
		}

		var mcid cid.Cid

		// if delegated-from address has been provided, treat as a TransferFrom.
		if dfrom := cctx.String("delegated-from"); dfrom != "" {
			_, _ = fmt.Fprintf(w, "delegated transfer of %d units from %s to %s via %s\n", amount, dfrom, recipient, sender)
			dfrom, err := address.NewFromString(dfrom)
			if err != nil {
				return fmt.Errorf("failed to parse address %s: %w", dfrom, err)
			}
			mcid, err = api.TokenTransferFrom(ctx, token, dfrom, sender, recipient, amount)
			if err != nil {
				return fmt.Errorf("transfer from failed: %w", err)
			}
		} else {
			_, _ = fmt.Fprintf(w, "transfer of %d units from %s to %s\n", amount, sender, recipient)
			mcid, err = api.TokenTransfer(ctx, token, sender, recipient, amount)
			if err != nil {
				return fmt.Errorf("transfer failed: %w", err)
			}
		}

		_, _ = fmt.Fprintf(w, "message CID: %s\n", mcid)
		_, _ = fmt.Fprintf(w, "awaiting %d confirmations...\n", confidence)

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		return processResult(w, result)
	},
}

var tokenApproveCmd = &cli.Command{
	Name:      "approve",
	Usage:     "Approve a delegate to transfer funds on behalf of an account",
	ArgsUsage: "<delegate address> <amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "holder address, and signer; will use the wallet default address if not provided",
		},
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		w := cctx.App.Writer

		if cctx.NArg() != 2 {
			return ShowHelp(cctx, fmt.Errorf("must specify delegate and amount"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		var (
			confidence = uint64(cctx.Int("confidence"))

			holder   address.Address
			delegate address.Address
			token    address.Address
			amount   abi.TokenAmount
		)

		// holder address.
		if c := cctx.String("holder"); c == "" {
			holder, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			holder, err = address.NewFromString(c)
			if err != nil {
				return fmt.Errorf("failed to parse holder address: %w", err)
			}
		}

		// token address.
		token, err = address.NewFromString(cctx.String("token"))
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", token, err)
		}

		// delegate address.
		delegate, err = address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		// amount.
		if amount, err = types.BigFromString(cctx.Args().Get(1)); err != nil {
			return fmt.Errorf("failed to parse amount: %w", err)
		}

		_, _ = fmt.Fprintf(w, "approving %s to spend %d units on behalf of %s...\n", delegate, amount, holder)

		mcid, err := api.TokenApprove(ctx, token, holder, delegate, amount)
		if err != nil {
			return fmt.Errorf("approval failed: %w", err)
		}

		_, _ = fmt.Fprintf(w, "message CID: %s\n", mcid)
		_, _ = fmt.Fprintf(w, "awaiting %d confirmations...\n", confidence)

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		return processResult(w, result)
	},
}

func processResult(w io.Writer, result *api.MsgLookup) error {
	// check it executed successfully
	if code := result.Receipt.ExitCode; code != 0 {
		msg := fmt.Sprintf("transaction failed; exit code: %d", code)
		_, _ = fmt.Fprintln(w, msg)
		return errors.New(msg)
	}
	_, _ = fmt.Fprintln(w, "transaction succeeded")
	return nil
}
