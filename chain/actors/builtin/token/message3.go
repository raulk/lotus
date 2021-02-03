package token

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	init0 "github.com/filecoin-project/specs-actors/actors/builtin/init"
	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	"github.com/filecoin-project/specs-actors/v3/actors/builtin/token"

	"github.com/filecoin-project/lotus/chain/actors"
	init_ "github.com/filecoin-project/lotus/chain/actors/builtin/init"
	"github.com/filecoin-project/lotus/chain/types"
)

type message3 struct {
	from address.Address
}

var _ MessageBuilder = message3{}

func (m message3) Create(info *Info) (*types.Message, error) {
	params := &token.ConstructorParams{
		Name:          info.Name,
		Symbol:        info.Symbol,
		Icon:          info.Icon,
		Decimals:      info.Decimals,
		TotalSupply:   info.TotalSupply,
		SystemAccount: m.from,
	}

	enc, err := actors.SerializeParams(params)
	if err != nil {
		return nil, err
	}

	execParams := &init0.ExecParams{
		CodeCID:           builtin3.TokenActorCodeID,
		ConstructorParams: enc,
	}

	enc, err = actors.SerializeParams(execParams)
	if err != nil {
		return nil, err
	}

	return &types.Message{
		To:     init_.Address,
		From:   m.from,
		Method: builtin0.MethodsInit.Exec,
		Params: enc,
		Value:  big.Zero(),
	}, nil
}

func (m message3) Transfer(tokenAddr address.Address, to address.Address, amount abi.TokenAmount) (*types.Message, error) {
	params := &token.TransferParams{
		To:    to,
		Value: amount,
	}

	enc, err := actors.SerializeParams(params)
	if err != nil {
		return nil, err
	}

	return &types.Message{
		To:     tokenAddr,
		From:   m.from,
		Method: builtin3.MethodsToken.Transfer,
		Params: enc,
		Value:  big.Zero(),
	}, nil
}

func (m message3) TransferFrom(tokenAddr address.Address, holder, to address.Address, amount abi.TokenAmount) (*types.Message, error) {
	params := &token.TransferFromParams{
		From:  holder,
		To:    to,
		Value: amount,
	}

	enc, err := actors.SerializeParams(params)
	if err != nil {
		return nil, err
	}

	return &types.Message{
		To:     tokenAddr,
		From:   m.from,
		Method: builtin3.MethodsToken.TransferFrom,
		Params: enc,
		Value:  big.Zero(),
	}, nil
}

func (m message3) Approve(tokenAddr address.Address, spender address.Address, amount abi.TokenAmount) (*types.Message, error) {
	params := &token.ApproveParams{
		Approvee: spender,
		Value:    amount,
	}

	enc, err := actors.SerializeParams(params)
	if err != nil {
		return nil, err
	}

	return &types.Message{
		To:     tokenAddr,
		From:   m.from,
		Method: builtin3.MethodsToken.Approve,
		Params: enc,
		Value:  big.Zero(),
	}, nil
}
