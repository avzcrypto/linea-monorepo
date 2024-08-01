package keccak

import (
	"testing"

	"github.com/consensys/zkevm-monorepo/prover/maths/field"
	"github.com/consensys/zkevm-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/zkevm-monorepo/prover/protocol/wizard"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/hash/generic/testdata"
	"github.com/stretchr/testify/assert"
)

func MakeTestCaseKeccak(t *testing.T, c []makeTestCase) (
	define wizard.DefineFunc,
	prover wizard.ProverStep,
) {
	mod := &keccakHash{}
	maxNumKeccakF := 12
	gdms := make([]generic.GenDataModule, len(c))

	define = func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		for i := range gdms {
			gdms[i] = testdata.CreateGenDataModule(comp, c[i].Name, c[i].Size)
		}

		inp := KeccakInput{
			Settings: &Settings{
				MaxNumKeccakf: maxNumKeccakF,
			},
			Providers: gdms,
		}
		mod = NewKeccak(comp, inp)
	}

	prover = func(run *wizard.ProverRuntime) {
		for i := range gdms {
			testdata.GenerateAndAssignGenDataModule(run, &gdms[i], c[i].HashNum, c[i].ToHash)
		}
		mod.Run(run)

		// check the hash result
		permTrace := GenerateTrace(mod.Provider.ScanStreams(run))
		hi := mod.HashHi.GetColAssignment(run).IntoRegVecSaveAlloc()
		lo := mod.HashLo.GetColAssignment(run).IntoRegVecSaveAlloc()
		for i, expectedHash := range permTrace.HashOutPut {
			// hashHi := hash[:16] ,  hashLo := hash[16:]
			gotHashHi := hi[i].Bytes()
			gotHashLo := lo[i].Bytes()
			assert.Equal(t, expectedHash[:16], gotHashHi[16:])
			assert.Equal(t, expectedHash[16:], gotHashLo[16:])
		}

		for i := len(permTrace.HashOutPut); i < len(hi); i++ {
			assert.Equal(t, field.Zero(), hi[i])
			assert.Equal(t, field.Zero(), lo[i])
		}
	}

	return define, prover
}

func TestKeccak(t *testing.T) {
	definer, prover := MakeTestCaseKeccak(t, testCases)
	comp := wizard.Compile(definer, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

type makeTestCase struct {
	Name    string
	Size    int
	HashNum []int
	ToHash  []int
}

var testCases = []makeTestCase{
	{
		Name:    "GenDataModule1",
		Size:    8,
		HashNum: []int{1, 1, 1, 1, 2},
		ToHash:  []int{1, 0, 1, 0, 1},
	},
	{
		Name:    "GenDataModule2",
		Size:    16,
		HashNum: []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3},
		ToHash:  []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0},
	},
	{
		Name:    "GenDataModule3",
		Size:    32,
		HashNum: []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3, 4, 4, 4, 4, 4},
		ToHash:  []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1},
	},
}