package full

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"go.uber.org/fx"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors"
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

func (t *TokenAPI) TokenCreate(ctx context.Context, creator address.Address, info *token.Info) (cid.Cid, error) {
	return t.pushMessage(ctx, creator, func(mb token.MessageBuilder) (*types.Message, error) {
		return mb.Create(info)
	})
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

func (t *TokenAPI) TokenGetHolders(ctx context.Context, tokenAddr address.Address) (map[api.TokenHolder]abi.TokenAmount, error) {
	actor, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	state, err := token.Load(t.Chain.Store(ctx), actor)
	if err != nil {
		return nil, fmt.Errorf("failed to load actor state: %w", err)
	}

	ret := make(map[api.TokenHolder]abi.TokenAmount)
	err = state.ForEachHolder(func(holder address.Address, balance abi.TokenAmount) error {
		ret[holder] = balance
		return nil
	})
	return ret, err
}

func (t *TokenAPI) TokenGetSpendersOf(ctx context.Context, tokenAddr address.Address, holder address.Address) (map[api.TokenSpender]abi.TokenAmount, error) {
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

func (t *TokenAPI) TokenTransfer(ctx context.Context, tokenAddr address.Address, from, to address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	_, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	return t.pushMessage(ctx, from, func(mb token.MessageBuilder) (*types.Message, error) {
		return mb.Transfer(tokenAddr, to, amount)
	})
}

func (t *TokenAPI) TokenTransferFrom(ctx context.Context, tokenAddr address.Address, holder, from, to address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	_, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	return t.pushMessage(ctx, from, func(mb token.MessageBuilder) (*types.Message, error) {
		return mb.TransferFrom(tokenAddr, holder, to, amount)
	})
}

func (t *TokenAPI) TokenApprove(ctx context.Context, tokenAddr address.Address, holder, spender address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	_, err := t.StateAPI.StateGetActor(ctx, tokenAddr, types.EmptyTSK)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to load token actor at address %s: %w", tokenAddr, err)
	}

	return t.pushMessage(ctx, holder, func(mb token.MessageBuilder) (*types.Message, error) {
		return mb.Approve(tokenAddr, spender, amount)
	})
}

func (t *TokenAPI) pushMessage(ctx context.Context, from address.Address, fn func(mb token.MessageBuilder) (*types.Message, error)) (cid.Cid, error) {
	nver, err := t.StateAPI.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return cid.Undef, err
	}

	builder := token.Message(actors.VersionForNetwork(nver), from)
	msg, err := fn(builder)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to build message: %w", err)
	}

	// send the message out to the network
	smsg, err := t.MpoolAPI.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to push message: %w", err)
	}

	return smsg.Cid(), nil
}

func (t *TokenAPI) messageBuilder(ctx context.Context, from address.Address) (token.MessageBuilder, error) {
	nver, err := t.StateAPI.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	return token.Message(actors.VersionForNetwork(nver), from), nil
}
