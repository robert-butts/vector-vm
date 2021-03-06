package main

import (
	"fmt"
)

type ControlUnit32bit struct {
	data           *ControlUnitData
	ProgramCounter int64
}

/*
nThe Memory is 1 "BytesPerElement" larger than the number of PEs. This is so the CU may have its own memory.
*/
func NewControlUnit32bit(indexRegisters uint, processingElements uint, memoryBytesPerElement uint) ControlUnit {
	var cu ControlUnit32bit
	cu.data = NewControlUnitData(indexRegisters, processingElements, memoryBytesPerElement)
	return &cu
}

func (cu *ControlUnit32bit) Data() *ControlUnitData {
	return cu.data
}

func (cu *ControlUnit32bit) PrintMachine() {
	fmt.Println("Machine: 32bit")
	cu.data.PrintMachine()
}

func (cu *ControlUnit32bit) RunProgram(program Program) error {
	cu.ProgramCounter = 0
	for cu.ProgramCounter != int64(program.Size()) {
		pc := cu.ProgramCounter
		inst := program.At(pc)
		op := OpCode(inst[0])
		if !isMem(op) {
			param1 := inst[1]
			param2 := inst[2]
			param3 := inst[3]

			if cu.data.Verbose {
				fmt.Printf("Run() PC: %3d  IS: %5s  P1: %d  P2: %d  P3: %d\n", cu.ProgramCounter, op.String(), param1, param2, param3) // debug
			}
			cu.Execute(op, []byte{param1, param2, param3})
			if cu.data.Verbose {
				cu.data.PrintMachine() // debug
			}
		} else {
			param := inst[1]
			memParam := uint16(inst[2]) | uint16(inst[3])<<8
			if cu.data.Verbose {
				fmt.Printf("Run() PC: %3d  IS: %5s  P: %d  MP: %d\n", cu.ProgramCounter, op.String(), param, memParam) // debug
			}
			cu.ExecuteMem(op, param, memParam)
			if cu.data.Verbose {
				cu.data.PrintMachine() // debug
			}
		}
		cu.ProgramCounter++
	}
	return nil
}
func (cu *ControlUnit32bit) Run(file string) error {
	program, err := LoadProgram32bit(file)
	if err != nil {
		return err
	}
	return cu.RunProgram(program)
}

func (cu *ControlUnit32bit) ExecuteMem(op OpCode, param byte, memParam uint16) {
	switch op {
	case isLdx:
		cu.Ldx(param, memParam)
	case isStx:
		cu.Stx(param, memParam)
	case isCload:
		cu.Cload(memParam)
	case isCstore:
		cu.Cstore(memParam)
	case isLdxi:
		cu.Ldxi(param, memParam)
	case isIncx:
		cu.Incx(param, memParam)
	case isDecx:
		cu.Decx(param, memParam)
	case isMulx:
		cu.Mulx(param, memParam)
	}
}

/// @param params must have as many members as the instruction takes parameters
func (cu *ControlUnit32bit) Execute(instruction OpCode, params []byte) {
	switch instruction {
	case isCmpx:
		cu.Cmpx(params[0], params[1], params[2])
	case isCbcast:
		cu.Cbcast()
	case isLod:
		cu.Lod(params[0], params[1])
	case isSto:
		cu.Sto(params[0], params[1])
	case isAdd:
		cu.Add(params[0], params[1])
	case isSub:
		cu.Sub(params[0], params[1])
	case isMul:
		cu.Mul(params[0], params[1])
	case isDiv:
		cu.Div(params[0], params[1])
	case isBcast:
		cu.Bcast(params[0])
	case isMov:
		cu.Mov(RegisterType(params[0]), RegisterType(params[1])) ///< @todo change to be multiple 'instructions' ?
	case isRadd:
		cu.Radd()
	case isRsub:
		cu.Rsub()
	case isRmul:
		cu.Rmul()
	case isRdiv:
		cu.Rdiv()
	}
}

//
// control instructions
//
func (cu *ControlUnit32bit) Ldx(index byte, a uint16) {
	//	fmt.Printf("ldx: cu.data.index[%d] = cu.data.Memory[%d] (%d)"
	cu.data.IndexRegister[index] = cu.data.Memory[a]
}
func (cu *ControlUnit32bit) Stx(index byte, a uint16) {
	cu.data.Memory[a] = cu.data.IndexRegister[index]
}
func (cu *ControlUnit32bit) Ldxi(index byte, a uint16) {
	cu.data.IndexRegister[index] = int64(a)
}
func (cu *ControlUnit32bit) Incx(index byte, a uint16) {
	cu.data.IndexRegister[index] += int64(a)
}
func (cu *ControlUnit32bit) Decx(index byte, a uint16) {
	cu.data.IndexRegister[index] -= int64(a)
}
func (cu *ControlUnit32bit) Mulx(index byte, a uint16) {
	cu.data.IndexRegister[index] *= int64(a)
}
func (cu *ControlUnit32bit) Cload(index uint16) {
	cu.data.ArithmeticRegister = cu.data.Memory[index]
}
func (cu *ControlUnit32bit) Cstore(index uint16) {
	cu.data.Memory[index] = cu.data.ArithmeticRegister
}

/// @todo fix this to take a larger jump (a). Byte only allows for 256 instructions. That's not a very big program
func (cu *ControlUnit32bit) Cmpx(index byte, ix2 byte, a byte) {
	if cu.data.IndexRegister[index] < cu.data.IndexRegister[ix2] {
		cu.ProgramCounter = int64(a) - 1 // -1 because the PC will be incremented.
	}
}

// control broadcast. Broadcasts the Control's Arithmetic Register to every PE's Routing Register
func (cu *ControlUnit32bit) Cbcast() {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].RoutingRegister = cu.data.ArithmeticRegister
	}
}

// Block until all PE's are done
func (cu *ControlUnit32bit) Barrier() {
	for i := 0; i != len(cu.data.PE); i++ {
		<-cu.data.Done
	}
}

//
// vector instructions
//
func (cu *ControlUnit32bit) Lod(a byte, idx byte) {
	//	fmt.Printf("PE-Lod %d + %d (%d)\n", a, cu.data.IndexRegister[idx], idx)
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Lod <- ByteTuple{a, byte(cu.data.IndexRegister[idx])} ///< @todo is this ok? Should we be loading the index register somewhere else?
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Sto(a byte, idx byte) {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Sto <- ByteTuple{a, byte(cu.data.IndexRegister[idx])}
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Add(a byte, idx byte) {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Add <- ByteTuple{a, byte(cu.data.IndexRegister[idx])}
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Sub(a byte, idx byte) {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Sub <- ByteTuple{a, byte(cu.data.IndexRegister[idx])}
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Mul(a byte, idx byte) {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Mul <- ByteTuple{a, byte(cu.data.IndexRegister[idx])}
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Div(a byte, idx byte) {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Div <- ByteTuple{a, byte(cu.data.IndexRegister[idx])}
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Bcast(idx byte) {
	idx = byte(cu.data.IndexRegister[idx]) ///< @todo is this ok? Should we be loading the index register here?
	for i, _ := range cu.data.PE {
		if !cu.data.PE[i].Enabled {
			continue
		}
		cu.data.PE[i].RoutingRegister = cu.data.PE[idx].RoutingRegister
	}
}
func (cu *ControlUnit32bit) Mov(from RegisterType, to RegisterType) {
	/// @todo remove this? speed for safety?
	if from == to {
		return
	}
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Mov <- ByteTuple{byte(from), byte(to)}
	}
	cu.Barrier()
}

func (cu *ControlUnit32bit) Radd() {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Radd <- true
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Rsub() {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Rsub <- true
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Rmul() {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Rmul <- true
	}
	cu.Barrier()
}
func (cu *ControlUnit32bit) Rdiv() {
	for i, _ := range cu.data.PE {
		cu.data.PE[i].Rdiv <- true
	}
	cu.Barrier()
}
