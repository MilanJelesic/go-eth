package ethchain

import (
	_ "bytes"
	"fmt"
	"github.com/ethereum/eth-go/ethutil"
	_ "github.com/obscuren/secp256k1-go"
	_ "math"
	"math/big"
)

var (
	GasStep    = big.NewInt(1)
	GasSha     = big.NewInt(20)
	GasSLoad   = big.NewInt(20)
	GasSStore  = big.NewInt(100)
	GasBalance = big.NewInt(20)
	GasCreate  = big.NewInt(100)
	GasCall    = big.NewInt(20)
	GasMemory  = big.NewInt(1)
)

type Vm struct {
	txPool *TxPool
	// Stack for processing contracts
	stack *Stack
	// non-persistent key/value memory storage
	mem map[string]*big.Int

	vars RuntimeVars

	state *State
}

type RuntimeVars struct {
	Origin      []byte
	BlockNumber uint64
	PrevHash    []byte
	Coinbase    []byte
	Time        int64
	Diff        *big.Int
	TxData      []string
}

func NewVm(state *State, vars RuntimeVars) *Vm {
	return &Vm{vars: vars, state: state}
}

var Pow256 = ethutil.BigPow(2, 256)

func (vm *Vm) RunClosure(closure *Closure, hook DebugHook) []byte {
	// If the amount of gas supplied is less equal to 0
	if closure.Gas.Cmp(big.NewInt(0)) <= 0 {
		// TODO Do something
	}

	// Memory for the current closure
	mem := &Memory{}
	// New stack (should this be shared?)
	stack := NewStack()
	// Instruction pointer
	pc := big.NewInt(0)
	// Current step count
	step := 0
	// The base for all big integer arithmetic
	base := new(big.Int)

	/*
		if ethutil.Config.Debug {
			ethutil.Config.Log.Debugf("#   op\n")
		}
	*/

	for {
		step++
		// Get the memory location of pc
		val := closure.Get(pc)
		// Get the opcode (it must be an opcode!)
		op := OpCode(val.Uint())
		/*
			if ethutil.Config.Debug {
				ethutil.Config.Log.Debugf("%-3d %-4s", pc, op.String())
			}
		*/

		// TODO Get each instruction cost properly
		gas := new(big.Int)
		useGas := func(amount *big.Int) {
			gas.Add(gas, amount)
		}

		switch op {
		case oSHA3:
			useGas(GasSha)
		case oSLOAD:
			useGas(GasSLoad)
		case oSSTORE:
			var mult *big.Int
			y, x := stack.Peekn()
			val := closure.GetMem(x)
			if val.IsEmpty() && len(y.Bytes()) > 0 {
				mult = ethutil.Big2
			} else if !val.IsEmpty() && len(y.Bytes()) == 0 {
				mult = ethutil.Big0
			} else {
				mult = ethutil.Big1
			}
			useGas(base.Mul(mult, GasSStore))
		case oBALANCE:
			useGas(GasBalance)
		case oCREATE:
			useGas(GasCreate)
		case oCALL:
			useGas(GasCall)
		case oMLOAD, oMSIZE, oMSTORE8, oMSTORE:
			useGas(GasMemory)
		default:
			useGas(GasStep)
		}

		if closure.Gas.Cmp(gas) < 0 {
			ethutil.Config.Log.Debugln("Insufficient gas", closure.Gas, gas)

			return closure.Return(nil)
		}

		switch op {
		case oLOG:
			stack.Print()
			mem.Print()
		case oSTOP: // Stop the closure
			return closure.Return(nil)

		// 0x20 range
		case oADD:
			x, y := stack.Popn()
			// (x + y) % 2 ** 256
			base.Add(x, y)
			base.Mod(base, Pow256)
			// Pop result back on the stack
			stack.Push(base)
		case oSUB:
			x, y := stack.Popn()
			// (x - y) % 2 ** 256
			base.Sub(x, y)
			base.Mod(base, Pow256)
			// Pop result back on the stack
			stack.Push(base)
		case oMUL:
			x, y := stack.Popn()
			// (x * y) % 2 ** 256
			base.Mul(x, y)
			base.Mod(base, Pow256)
			// Pop result back on the stack
			stack.Push(base)
		case oDIV:
			x, y := stack.Popn()
			// floor(x / y)
			base.Div(x, y)
			// Pop result back on the stack
			stack.Push(base)
		case oSDIV:
			x, y := stack.Popn()
			// n > 2**255
			if x.Cmp(Pow256) > 0 {
				x.Sub(Pow256, x)
			}
			if y.Cmp(Pow256) > 0 {
				y.Sub(Pow256, y)
			}
			z := new(big.Int)
			z.Div(x, y)
			if z.Cmp(Pow256) > 0 {
				z.Sub(Pow256, z)
			}
			// Push result on to the stack
			stack.Push(z)
		case oMOD:
			x, y := stack.Popn()
			base.Mod(x, y)
			stack.Push(base)
		case oSMOD:
			x, y := stack.Popn()
			// n > 2**255
			if x.Cmp(Pow256) > 0 {
				x.Sub(Pow256, x)
			}
			if y.Cmp(Pow256) > 0 {
				y.Sub(Pow256, y)
			}
			z := new(big.Int)
			z.Mod(x, y)
			if z.Cmp(Pow256) > 0 {
				z.Sub(Pow256, z)
			}
			// Push result on to the stack
			stack.Push(z)
		case oEXP:
			x, y := stack.Popn()
			base.Exp(x, y, Pow256)

			stack.Push(base)
		case oNEG:
			base.Sub(Pow256, stack.Pop())
			stack.Push(base)
		case oLT:
			x, y := stack.Popn()
			// x < y
			if x.Cmp(y) < 0 {
				stack.Push(ethutil.BigTrue)
			} else {
				stack.Push(ethutil.BigFalse)
			}
		case oGT:
			x, y := stack.Popn()
			// x > y
			if x.Cmp(y) > 0 {
				stack.Push(ethutil.BigTrue)
			} else {
				stack.Push(ethutil.BigFalse)
			}
		case oEQ:
			x, y := stack.Popn()
			// x == y
			if x.Cmp(y) == 0 {
				stack.Push(ethutil.BigTrue)
			} else {
				stack.Push(ethutil.BigFalse)
			}
		case oNOT:
			x := stack.Pop()
			if x.Cmp(ethutil.BigFalse) == 0 {
				stack.Push(ethutil.BigTrue)
			} else {
				stack.Push(ethutil.BigFalse)
			}

		// 0x10 range
		case oAND:
			x, y := stack.Popn()
			if (x.Cmp(ethutil.BigTrue) >= 0) && (y.Cmp(ethutil.BigTrue) >= 0) {
				stack.Push(ethutil.BigTrue)
			} else {
				stack.Push(ethutil.BigFalse)
			}

		case oOR:
			x, y := stack.Popn()
			if (x.Cmp(ethutil.BigInt0) >= 0) || (y.Cmp(ethutil.BigInt0) >= 0) {
				stack.Push(ethutil.BigTrue)
			} else {
				stack.Push(ethutil.BigFalse)
			}
		case oXOR:
			x, y := stack.Popn()
			stack.Push(base.Xor(x, y))
		case oBYTE:
			val, th := stack.Popn()
			if th.Cmp(big.NewInt(32)) < 0 {
				stack.Push(big.NewInt(int64(len(val.Bytes())-1) - th.Int64()))
			} else {
				stack.Push(ethutil.BigFalse)
			}

		// 0x20 range
		case oSHA3:
			size, offset := stack.Popn()
			data := mem.Get(offset.Int64(), size.Int64())

			stack.Push(ethutil.BigD(data))
		// 0x30 range
		case oADDRESS:
			stack.Push(ethutil.BigD(closure.Object().Address()))
		case oBALANCE:
			stack.Push(closure.Value)
		case oORIGIN:
			stack.Push(ethutil.BigD(vm.vars.Origin))
		case oCALLER:
			stack.Push(ethutil.BigD(closure.Callee().Address()))
		case oCALLVALUE:
			// FIXME: Original value of the call, not the current value
			stack.Push(closure.Value)
		case oCALLDATA:
			offset := stack.Pop()
			mem.Set(offset.Int64(), int64(len(closure.Args)), closure.Args)
		case oCALLDATASIZE:
			stack.Push(big.NewInt(int64(len(closure.Args))))
		case oGASPRICE:
			// TODO

		// 0x40 range
		case oPREVHASH:
			stack.Push(ethutil.BigD(vm.vars.PrevHash))
		case oCOINBASE:
			stack.Push(ethutil.BigD(vm.vars.Coinbase))
		case oTIMESTAMP:
			stack.Push(big.NewInt(vm.vars.Time))
		case oNUMBER:
			stack.Push(big.NewInt(int64(vm.vars.BlockNumber)))
		case oDIFFICULTY:
			stack.Push(vm.vars.Diff)
		case oGASLIMIT:
			// TODO

		// 0x50 range
		case oPUSH: // Push PC+1 on to the stack
			pc.Add(pc, ethutil.Big1)
			data := closure.Gets(pc, big.NewInt(32))
			val := ethutil.BigD(data.Bytes())

			// Push value to stack
			stack.Push(val)

			pc.Add(pc, big.NewInt(31))
			step++
		case oPUSH20:
			pc.Add(pc, ethutil.Big1)
			data := closure.Gets(pc, big.NewInt(20))
			val := ethutil.BigD(data.Bytes())

			// Push value to stack
			stack.Push(val)

			pc.Add(pc, big.NewInt(19))
			step++
		case oPOP:
			stack.Pop()
		case oDUP:
			stack.Push(stack.Peek())
		case oSWAP:
			x, y := stack.Popn()
			stack.Push(y)
			stack.Push(x)
		case oMLOAD:
			offset := stack.Pop()
			stack.Push(ethutil.BigD(mem.Get(offset.Int64(), 32)))
		case oMSTORE: // Store the value at stack top-1 in to memory at location stack top
			// Pop value of the stack
			val, mStart := stack.Popn()
			mem.Set(mStart.Int64(), 32, ethutil.BigToBytes(val, 256))
		case oMSTORE8:
			val, mStart := stack.Popn()
			base.And(val, new(big.Int).SetInt64(0xff))
			mem.Set(mStart.Int64(), 32, ethutil.BigToBytes(base, 256))
		case oSLOAD:
			loc := stack.Pop()
			val := closure.GetMem(loc)
			stack.Push(val.BigInt())
		case oSSTORE:
			val, loc := stack.Popn()
			closure.SetMem(loc, ethutil.NewValue(val))
		case oJUMP:
			pc = stack.Pop()
		case oJUMPI:
			cond, pos := stack.Popn()
			if cond.Cmp(ethutil.BigTrue) == 0 {
				pc = pos
			}
		case oPC:
			stack.Push(pc)
		case oMSIZE:
			stack.Push(big.NewInt(int64(mem.Len())))
		// 0x60 range
		case oCALL:
			// Closure addr
			addr := stack.Pop()
			// Pop gas and value of the stack.
			gas, value := stack.Popn()
			// Pop input size and offset
			inSize, inOffset := stack.Popn()
			// Pop return size and offset
			retSize, retOffset := stack.Popn()
			// Get the arguments from the memory
			args := mem.Get(inOffset.Int64(), inSize.Int64())
			// Fetch the contract which will serve as the closure body
			contract := vm.state.GetContract(addr.Bytes())
			// Create a new callable closure
			closure := NewClosure(closure, contract, contract.script, vm.state, gas, value)
			// Executer the closure and get the return value (if any)
			ret := closure.Call(vm, args, hook)

			mem.Set(retOffset.Int64(), retSize.Int64(), ret)
		case oRETURN:
			size, offset := stack.Popn()
			ret := mem.Get(offset.Int64(), size.Int64())

			return closure.Return(ret)
		case oSUICIDE:
			/*
				recAddr := stack.Pop().Bytes()
				// Purge all memory
				deletedMemory := contract.state.Purge()
				// Add refunds to the pop'ed address
				refund := new(big.Int).Mul(StoreFee, big.NewInt(int64(deletedMemory)))
				account := state.GetAccount(recAddr)
				account.Amount.Add(account.Amount, refund)
				// Update the refunding address
				state.UpdateAccount(recAddr, account)
				// Delete the contract
				state.trie.Update(string(addr), "")

				ethutil.Config.Log.Debugf("(%d) => %x\n", deletedMemory, recAddr)
				break out
			*/
		default:
			ethutil.Config.Log.Debugf("Invalid opcode %x\n", op)

			return closure.Return(nil)
		}

		pc.Add(pc, ethutil.Big1)

		if hook != nil {
			hook(step-1, op, mem, stack)
		}
	}
}

func Disassemble(script []byte) (asm []string) {
	pc := new(big.Int)
	for {
		if pc.Cmp(big.NewInt(int64(len(script)))) >= 0 {
			return
		}

		// Get the memory location of pc
		val := script[pc.Int64()]
		// Get the opcode (it must be an opcode!)
		op := OpCode(val)

		asm = append(asm, fmt.Sprintf("%v", op))

		switch op {
		case oPUSH: // Push PC+1 on to the stack
			pc.Add(pc, ethutil.Big1)
			data := script[pc.Int64() : pc.Int64()+32]
			val := ethutil.BigD(data)

			var b []byte
			if val.Int64() == 0 {
				b = []byte{0}
			} else {
				b = val.Bytes()
			}

			asm = append(asm, fmt.Sprintf("0x%x", b))

			pc.Add(pc, big.NewInt(31))
		case oPUSH20:
			pc.Add(pc, ethutil.Big1)
			data := script[pc.Int64() : pc.Int64()+20]
			val := ethutil.BigD(data)
			var b []byte
			if val.Int64() == 0 {
				b = []byte{0}
			} else {
				b = val.Bytes()
			}

			asm = append(asm, fmt.Sprintf("0x%x", b))

			pc.Add(pc, big.NewInt(19))
		}

		pc.Add(pc, ethutil.Big1)
	}

	return
}
