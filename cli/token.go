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
	},
}

var tokenCreateCmd = &cli.Command{
	Name:  "create",
	Usage: "Create a new token actor",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "creator",
			Usage: "the wallet address to create the actor from",
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
			Name:     "total supply",
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
				return err
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
			return err
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
	Usage:     "Retrieve the basic of a token actor",
	ArgsUsage: "[address]",
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return ShowHelp(cctx, fmt.Errorf("must specify address of token actor to retrieve info of"))
		}

		api, closer, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)

		addr, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
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
