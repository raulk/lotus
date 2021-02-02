package token

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v3/actors/builtin"
	"github.com/filecoin-project/specs-actors/v3/actors/builtin/token"
	adt3 "github.com/filecoin-project/specs-actors/v3/actors/util/adt"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/lotus/chain/actors/adt"
)

var _ State = (*state3)(nil)

// load3 loads the actor state for a token v3 actor.
func load3(store adt.Store, root cid.Cid) (State, error) {
	out := state3{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type state3 struct {
	token.State
	store adt.Store
}

func (s *state3) TokenInfo() (*Info, error) {
	return s.State.TokenInfo(), nil
}

func (s *state3) BalanceOf(holder address.Address) (abi.TokenAmount, error) {
	return s.State.BalanceOf(s.store, holder)
}

func (s *state3) ApprovalsBy(holder address.Address) (map[address.Address]abi.TokenAmount, error) {
	m, err := adt3.AsMap(s.store, s.Approvals, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, fmt.Errorf("failed to load approvals table: %w", err)
	}
	var spendersCid cbg.CborCid
	found, err := m.Get(abi.AddrKey(holder), &spendersCid)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve approver: %w", err)
	}
	if !found {
		return nil, nil
	}
	spenders, err := adt3.AsMap(s.store, cid.Cid(spendersCid), builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve spenders: %w", err)
	}

	ret := make(map[address.Address]abi.TokenAmount)
	var available abi.TokenAmount
	err = spenders.ForEach(&available, func(k string) error {
		addr, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		ret[addr] = available
		return nil
	})
	return ret, err
}

func (s *state3) ForEachHolder(cb func(addr address.Address, balance abi.TokenAmount) error) error {
	m, err := adt3.AsMap(s.store, s.Balances, builtin.DefaultHamtBitwidth)
	if err != nil {
		return fmt.Errorf("failed to load balances table: %w", err)
	}

	var balance abi.TokenAmount
	return m.ForEach(&balance, func(k string) error {
		addr, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		return cb(addr, balance)
	})
}

func (s *state3) ForEachApproval(cb func(holder address.Address, spender address.Address, available abi.TokenAmount) error) error {
	m, err := adt3.AsMap(s.store, s.Approvals, builtin.DefaultHamtBitwidth)
	if err != nil {
		return fmt.Errorf("failed to load approvals table: %w", err)
	}

	var spendersCid cbg.CborCid
	return m.ForEach(&spendersCid, func(k string) error {
		holder, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		spenders, err := adt3.AsMap(s.store, cid.Cid(spendersCid), builtin.DefaultHamtBitwidth)
		if err != nil {
			return fmt.Errorf("failed to resolve spenders for holder %s: %w", holder, err)
		}
		var available abi.TokenAmount
		return spenders.ForEach(&available, func(k string) error {
			spender, err := address.NewFromBytes([]byte(k))
			if err != nil {
				return err
			}
			return cb(holder, spender, available)
		})
	})
}
