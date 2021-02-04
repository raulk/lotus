package api

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/lotus/chain/actors/builtin/token"
)

type TokenInfo = token.Info

type Holder struct {
	IDAddress     address.Address
	PubKeyAddress address.Address
	Balance       abi.TokenAmount
}

type TokenAPI interface {
	// TokenInfo returns the token's information.
	TokenInfo(ctx context.Context, token address.Address) (*TokenInfo, error)

	// TokenCreate creates a new token with the specified info.
	TokenCreate(ctx context.Context, creator address.Address, info *TokenInfo) (cid.Cid, error)

	// TokenBalanceOf returns the balance the holder has of the specified token.
	TokenBalanceOf(ctx context.Context, token address.Address, holder address.Address) (abi.TokenAmount, error)

	// TokenGetHolders returns all holders of the token, along with their balances.
	TokenGetHolders(ctx context.Context, token address.Address) ([]Holder, error)

	// TokenGetSpendersOf returns all addresses the holder has authorized to
	// spend on their behalf, along with the available amounts.
	TokenGetSpendersOf(ctx context.Context, token address.Address, holder address.Address) (map[string]abi.TokenAmount, error)

	// TokenTransfer sends the specified amount of tokens from one address to
	// another.
	TokenTransfer(ctx context.Context, token address.Address, from, to address.Address, amount abi.TokenAmount) (cid.Cid, error)

	// TokenTransferFrom sends the specified amount of tokens from the holder
	// account to the destination account, via the authorized spender account.
	TokenTransferFrom(ctx context.Context, token address.Address, holder, from, to address.Address, amount abi.TokenAmount) (cid.Cid, error)

	// TokenApprove approves the specified spender to transfer token from
	// the holder account, up to the specified amount.
	TokenApprove(ctx context.Context, token address.Address, holder, spender address.Address, amount abi.TokenAmount) (cid.Cid, error)
}
