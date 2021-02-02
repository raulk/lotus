package token

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lotus/chain/actors/adt"
	"github.com/filecoin-project/lotus/chain/actors/builtin"
	"github.com/filecoin-project/lotus/chain/types"
)

func init() {
	builtin.RegisterActorState(builtin3.TokenActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load3(store, root)
	})
}

// Load returns an abstract copy of the token actor state, regardless of
// the actor version.
func Load(store adt.Store, act *types.Actor) (State, error) {
	switch act.Code {
	case builtin3.TokenActorCodeID:
		return load3(store, act.Head)
	}
	return nil, xerrors.Errorf("unknown actor code %s", act.Code)
}

// State is an abstract version of the token actor's state that works across
// versions.
type State interface {
	cbor.Marshaler

	// Name returns the name of this token.
	Name() string

	// Symbol returns the symbol of this token.
	Symbol() string

	// Decimals returns the decimals places for this token.
	Decimals() uint64

	// TotalSupply returns the total supply for this token.
	TotalSupply() abi.TokenAmount

	// BalanceOf returns the balance of
	BalanceOf(holder address.Address) (abi.TokenAmount, error)

	// ApprovalsBy returns the approvals that an address has made to spenders,
	// specifying the available amount to spend.
	ApprovalsBy(holder address.Address) (map[address.Address]abi.TokenAmount, error)

	// ForEachHolder iterates through the holdings map and invokes the callback
	// for every entry.
	//
	// TODO: document error behaviour.
	ForEachHolder(cb func(holder address.Address, balance abi.TokenAmount) error) error

	// ForEachApproval iterates through the approvals map and invokes the
	// callback for every entry.
	//
	// TODO: document error behaviour.
	ForEachApproval(cb func(holder address.Address, spender address.Address, available abi.TokenAmount) error) error
}
