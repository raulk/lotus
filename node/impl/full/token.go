package full

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"go.uber.org/fx"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors/builtin/token"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
)

type TokenAPI struct {
	fx.In

	Chain    *store.ChainStore
	StateAPI StateAPI
	MpoolAPI MpoolAPI
}

var _ api.TokenAPI = (*TokenAPI)(nil)

func (t *TokenAPI) TokenInfo(ctx context.Context, tokenAddr address.Address) (*token.Info, error) {
	actor, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	state, err := token.Load(t.Chain.Store(ctx), actor)
	if err != nil {
		return nil, fmt.Errorf("failed to load actor state: %w", err)
	}

	return state.TokenInfo()
}

func (t *TokenAPI) TokenBalanceOf(ctx context.Context, tokenAddr address.Address, holder address.Address) (abi.TokenAmount, error) {
	actor, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	state, err := token.Load(t.Chain.Store(ctx), actor)
	if err != nil {
		return big.Zero(), fmt.Errorf("failed to load actor state: %w", err)
	}

	return state.BalanceOf(holder)
}

func (t *TokenAPI) TokenGetHolders(ctx context.Context, tokenAddr address.Address) (map[api.Holder]abi.TokenAmount, error) {
	actor, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	state, err := token.Load(t.Chain.Store(ctx), actor)
	if err != nil {
		return nil, fmt.Errorf("failed to load actor state: %w", err)
	}

	ret := make(map[api.Holder]abi.TokenAmount)
	err = state.ForEachHolder(func(holder address.Address, balance abi.TokenAmount) error {
		ret[holder] = balance
		return nil
	})
	return ret, err
}

func (t *TokenAPI) TokenGetSpendersOf(ctx context.Context, tokenAddr address.Address, holder address.Address) (map[api.Spender]abi.TokenAmount, error) {
	actor, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	state, err := token.Load(t.Chain.Store(ctx), actor)
	if err != nil {
		return nil, fmt.Errorf("failed to load actor state: %w", err)
	}

	return state.ApprovalsBy(holder)
}

func (t *TokenAPI) TokenTransfer(ctx context.Context, token address.Address, from, to address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	panic("implement me")
}

func (t *TokenAPI) TokenTransferFrom(ctx context.Context, token address.Address, holder, from, to address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	panic("implement me")
}

func (t *TokenAPI) TokenApprove(ctx context.Context, token address.Address, holder, spender address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	panic("implement me")
}
