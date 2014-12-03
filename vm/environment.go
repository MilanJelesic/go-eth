package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/ethutil"
	"github.com/ethereum/go-ethereum/state"
)

type Environment interface {
	State() *state.State

	Origin() []byte
	BlockNumber() *big.Int
	PrevHash() []byte
	Coinbase() []byte
	Time() int64
	Difficulty() *big.Int
	BlockHash() []byte
	GasLimit() *big.Int
	Transfer(from, to Account, amount *big.Int) error
	AddLog(*state.Log)

	Depth() int
	SetDepth(i int)

	Call(me ClosureRef, addr, data []byte, gas, price, value *big.Int) ([]byte, error)
	CallCode(me ClosureRef, addr, data []byte, gas, price, value *big.Int) ([]byte, error)
	Create(me ClosureRef, addr, data []byte, gas, price, value *big.Int) ([]byte, error, ClosureRef)
}

type Object interface {
	GetStorage(key *big.Int) *ethutil.Value
	SetStorage(key *big.Int, value *ethutil.Value)
}

type Account interface {
	SubBalance(amount *big.Int)
	AddBalance(amount *big.Int)
	Balance() *big.Int
}

// generic transfer method
func Transfer(from, to Account, amount *big.Int) error {
	if from.Balance().Cmp(amount) < 0 {
		return errors.New("Insufficient balance in account")
	}

	from.SubBalance(amount)
	to.AddBalance(amount)

	return nil
}
