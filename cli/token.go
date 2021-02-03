package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	init3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/init"

	"github.com/filecoin-project/lotus/chain/actors/builtin/token"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/go-address"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
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
		tokenTransferFromCmd,
		tokenApproveCmd,
	},
}

var tokenCreateCmd = &cli.Command{
	Name:  "create",
	Usage: "Create a new token actor",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "creator",
			Usage: "the wallet address to create the actor from; will use the wallet default address if not provided",
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

			creator address.Address
			supply  abi.TokenAmount
			icon    []byte
		)

		if c := cctx.String("creator"); c == "" {
			creator, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			creator, err = address.NewFromString(c)
			if err != nil {
				return fmt.Errorf("failed to parse creator address: %w", err)
			}
		}

		if supply, err = types.BigFromString(cctx.String("supply")); err != nil {
			return fmt.Errorf("failed to parse supply: %w", err)
		}

		if icon, err = base64.StdEncoding.DecodeString(iconb64); err != nil {
			return fmt.Errorf("failed to decode base64 icon: %w", err)
		}

		mcid, err := api.TokenCreate(ctx, creator, &token.Info{
			Name:        name,
			Symbol:      symbol,
			Decimals:    decimals,
			Icon:        icon,
			TotalSupply: supply,
		})

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		// check it executed successfully
		if result.Receipt.ExitCode != 0 {
			_, _ = fmt.Fprintln(cctx.App.Writer, "actor creation failed!")
			return err
		}

		// get address of newly created miner
		var ret init3.ExecReturn
		if err := ret.UnmarshalCBOR(bytes.NewReader(result.Receipt.Return)); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cctx.App.Writer, "Created new token actor: ", ret.IDAddress, ret.RobustAddress)
		return nil
	},
}

var tokenInfoCmd = &cli.Command{
	Name:      "info",
	Usage:     "Retrieve the basic info of a token actor",
	ArgsUsage: "<address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return ShowHelp(cctx, fmt.Errorf("must specify address of token actor"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		addr, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", addr, err)
		}

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
	ArgsUsage: "<token address> <holders...>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() < 2 {
			return ShowHelp(cctx, fmt.Errorf("must specify address of token actor and at least one holder address"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		addrs := make([]address.Address, 0, cctx.NArg())
		for _, a := range cctx.Args().Slice() {
			addr, err := address.NewFromString(a)
			if err != nil {
				return fmt.Errorf("failed to parse address %s: %w", a, err)
			}
			addrs = append(addrs, addr)
		}

		token := addrs[0]
		holders := addrs[1:]

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
	Name:      "holders",
	Usage:     "Retrieve all token holders and their balances",
	ArgsUsage: "<token address>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return ShowHelp(cctx, fmt.Errorf("must specify address of token actor"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		addr, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", addr, err)
		}

		holders, err := api.TokenGetHolders(ctx, addr)
		if err != nil {
			return fmt.Errorf("failed to get holders: %w", err)
		}

		for holder, balance := range holders {
			_, _ = fmt.Fprintln(cctx.App.Writer, holder, balance)
		}

		return nil
	},
}

var tokenDelegationsCmd = &cli.Command{
	Name:      "delegations",
	Usage:     "Retrieve all token spending delegations from the provided holder address for the provided token",
	ArgsUsage: "<token address> <holder>",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return ShowHelp(cctx, fmt.Errorf("must specify addresses of token actor and the holder"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		tokenAddr, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", tokenAddr, err)
		}

		holderAddr, err := address.NewFromString(cctx.Args().Get(1))
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", holderAddr, err)
		}

		delegations, err := api.TokenGetSpendersOf(ctx, tokenAddr, holderAddr)
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
	Usage:     "Transfer a token balance",
	ArgsUsage: "<recipient> <amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "sender address; will use the wallet default address if not provided",
		},
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
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

			from   address.Address
			to     address.Address
			token  address.Address
			amount abi.TokenAmount
		)

		// sender address.
		if c := cctx.String("from"); c == "" {
			from, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			from, err = address.NewFromString(c)
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
		to, err = address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		// amount.
		if amount, err = types.BigFromString(cctx.Args().Get(1)); err != nil {
			return fmt.Errorf("failed to parse amount: %w", err)
		}

		mcid, err := api.TokenTransfer(ctx, token, from, to, amount)

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		// check it executed successfully
		if result.Receipt.ExitCode != 0 {
			_, _ = fmt.Fprintln(cctx.App.Writer, "transaction failed")
			return err
		}

		return nil
	},
}

var tokenTransferFromCmd = &cli.Command{
	Name:      "transfer-from",
	Usage:     "Transfer a token balance via a delegation",
	ArgsUsage: "<recipient> <amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "token holder address",
		},
		&cli.StringFlag{
			Name:     "sender",
			Usage:    "address of the delegate that's approved to spend the funds; will use the wallet default address if not provided",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
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

			from   address.Address
			to     address.Address
			sender address.Address
			token  address.Address
			amount abi.TokenAmount
		)

		// token holder address.
		if c := cctx.String("from"); c == "" {
			from, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			from, err = address.NewFromString(c)
			if err != nil {
				return fmt.Errorf("failed to parse address %s: %w", from, err)
			}
		}

		// delegate address.
		sender, err = address.NewFromString(cctx.String("sender"))
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", sender, err)
		}

		// token address.
		token, err = address.NewFromString(cctx.String("token"))
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", token, err)
		}

		// recipient address.
		to, err = address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		// amount.
		if amount, err = types.BigFromString(cctx.Args().Get(1)); err != nil {
			return fmt.Errorf("failed to parse amount: %w", err)
		}

		mcid, err := api.TokenTransferFrom(ctx, token, from, sender, to, amount)

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		// check it executed successfully
		if result.Receipt.ExitCode != 0 {
			_, _ = fmt.Fprintln(cctx.App.Writer, "transaction failed")
			return err
		}

		return nil
	},
}

var tokenApproveCmd = &cli.Command{
	Name:      "approve",
	Usage:     "Approve a delegate to transfer funds on behalf of an account",
	ArgsUsage: "<delegate address> <amount>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "from",
			Usage: "holder address; will use the wallet default address if not provided",
		},
		&cli.StringFlag{
			Name:     "token",
			Usage:    "token actor address",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
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

			from     address.Address
			delegate address.Address
			token    address.Address
			amount   abi.TokenAmount
		)

		// sender address.
		if c := cctx.String("from"); c == "" {
			from, err = api.WalletDefaultAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to get wallet default address: %w", err)
			}
		} else {
			from, err = address.NewFromString(c)
			if err != nil {
				return fmt.Errorf("failed to parse creator address: %w", err)
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

		mcid, err := api.TokenApprove(ctx, token, from, delegate, amount)

		// wait for it to get mined into a block
		result, err := api.StateWaitMsg(ctx, mcid, confidence)
		if err != nil {
			return fmt.Errorf("failed to wait for message: %w", err)
		}

		// check it executed successfully
		if result.Receipt.ExitCode != 0 {
			_, _ = fmt.Fprintln(cctx.App.Writer, "transaction failed")
			return err
		}

		return nil
	},
}
