package token

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"

	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/types"
)

var Methods = builtin3.MethodsMultisig

func Message(version actors.Version, from address.Address) MessageBuilder {
	switch version {
	case actors.Version3:
		return message3{from}
	default:
		panic(fmt.Sprintf("unsupported actors version: %d", version))
	}
}

type MessageBuilder interface {
	// Create produces a message to construct a new token actor.
	Create(info *Info) (*types.Message, error)

	// Transfer produces a message to transfer a token amount to another account.
	Transfer(token address.Address, to address.Address, amount abi.TokenAmount) (*types.Message, error)

	// TransferFrom produces a message to transfer a token amount to another account, via a delegation.
	TransferFrom(token address.Address, holder, to address.Address, amount abi.TokenAmount) (*types.Message, error)

	// Approve produces a message to approve another account as a spender for the sender.
	Approve(token address.Address, spender address.Address, amount abi.TokenAmount) (*types.Message, error)
}
