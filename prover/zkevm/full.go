package zkevm

import (
	"sync"

	"github.com/consensys/zkevm-monorepo/prover/config"
	"github.com/consensys/zkevm-monorepo/prover/crypto/ringsis"
	"github.com/consensys/zkevm-monorepo/prover/protocol/compiler"
	"github.com/consensys/zkevm-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/zkevm-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/zkevm-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/zkevm-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/zkevm-monorepo/prover/protocol/wizard"
	"github.com/consensys/zkevm-monorepo/prover/utils"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/ecarith"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/ecdsa"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/ecpair"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/hash/keccak"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/hash/sha2"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/modexp"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/statemanager"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/statemanager/accumulator"
)

const (
	// @TODO: the keccak limits are hardcoded currently, in the future we should
	// instead take the limits from the trace limit file. Note, neither of these
	// limits are actually enforced by the prover as the keccak module is not
	// connected to the rest of the arithmetization. Thus, it is easy to just
	// ignore the overflowing keccaks and the state merkle-proofs.
	keccakLimit      = 1 << 13
	merkleProofLimit = 1 << 13
)

var (
	fullZkEvm     *ZkEvm
	onceFullZkEvm = sync.Once{}

	// This is the SIS instance, that has been found to minimize the overhead of
	// recursion. It is changed w.r.t to the estimated because the estimated one
	// allows for 10 bits limbs instead of just 8. But since the current state
	// of the self-recursion currently relies on the number of limbs to be a
	// power of two, we go with this one although it overshoots our security
	// level target.
	sisInstance = ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}

	// This is the compilation suite in use for the full prover
	fullCompilationSuite = compilationSuite{
		// logdata.Log("initial-wizard"),
		mimc.CompileMiMC,
		compiler.Arcane(1<<10, 1<<19, false),
		vortex.Compile(
			2,
			vortex.ForceNumOpenedColumns(256),
			vortex.WithSISParams(&sisInstance),
		),
		// logdata.Log("post-vortex-1"),

		// First round of self-recursion
		selfrecursion.SelfRecurse,
		// logdata.Log("post-selfrecursion-1"),
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(1<<10, 1<<18, false),
		vortex.Compile(
			2,
			vortex.ForceNumOpenedColumns(256),
			vortex.WithSISParams(&sisInstance),
		),
		// logdata.Log("post-vortex-2"),

		// Second round of self-recursion
		selfrecursion.SelfRecurse,
		// logdata.Log("post-selfrecursion-2"),
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(1<<10, 1<<16, false),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(64),
			vortex.WithSISParams(&sisInstance),
		),

		// Fourth round of self-recursion
		// logdata.Log("post-vortex-3"),
		selfrecursion.SelfRecurse,
		// logdata.Log("post-selfrecursion-3"),
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(1<<10, 1<<13, false),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(64),
			vortex.ReplaceSisByMimc(),
		),
		// logdata.Log("post-vortex-4"),
	}
)

// FullZkEvm compiles the full prover zkEVM. It memoizes the results and
// returns it for all the subsequent calls. That is, it should not be called
// twice with different configuration parameters as it will always return the
// instance compiled with the parameters it received the first time. This
// behavior is motivated by the fact that the compilation process takes time
// and we don't want to spend the compilation time twice, plus in practice we
// won't need to call it with different configuration parameters.
func FullZkEvm(tl *config.TracesLimits) *ZkEvm {

	onceFullZkEvm.Do(func() {

		// @Alex: only set mandatory parameters here. aka, the one that are not
		// actually feature-gated.
		settings := Settings{
			Arithmetization: arithmetization.Settings{
				Traces: tl,
			},
			Statemanager: statemanager.Settings{
				AccSettings: accumulator.Settings{
					MaxNumProofs:    merkleProofLimit,
					Name:            "SM_ACCUMULATOR",
					MerkleTreeDepth: 40,
				},
				MiMCCodeHashSize: tl.Rom,
			},
			// The compilation suite itself is hard-coded and reflects the
			// actual full proof system.
			CompilationSuite: fullCompilationSuite,
			Metadata: wizard.VersionMetadata{
				Title:   "linea/evm-execution/full",
				Version: "beta-v1",
			},
			Keccak: keccak.Settings{
				MaxNumKeccakf: keccakLimit,
			},
			Ecdsa: ecdsa.Settings{
				MaxNbEcRecover:     tl.PrecompileEcrecoverEffectiveCalls,
				MaxNbTx:            tl.BlockTransactions,
				NbInputInstance:    4,
				NbCircuitInstances: utils.DivCeil(tl.PrecompileEcrecoverEffectiveCalls+tl.BlockTransactions, 4),
			},
			Modexp: modexp.Settings{
				MaxNbInstance256:  tl.PrecompileModexpEffectiveCalls,
				MaxNbInstance4096: 1,
			},
			Ecadd: ecarith.Limits{
				// 14 was found the right number to have just under 2^19 constraints
				// per circuit.
				NbInputInstances:   utils.DivCeil(tl.PrecompileEcaddEffectiveCalls, 28),
				NbCircuitInstances: 28,
			},
			Ecmul: ecarith.Limits{
				NbCircuitInstances: utils.DivCeil(tl.PrecompileEcmulEffectiveCalls, 6),
				NbInputInstances:   6,
			},
			Ecpair: ecpair.Limits{
				NbMillerLoopInputInstances:   1,
				NbMillerLoopCircuits:         tl.PrecompileEcpairingMillerLoops,
				NbFinalExpInputInstances:     1,
				NbFinalExpCircuits:           tl.PrecompileEcpairingEffectiveCalls,
				NbG2MembershipInputInstances: 6,
				NbG2MembershipCircuits:       utils.DivCeil(tl.PrecompileEcpairingG2MembershipCalls, 6),
			},
			Sha2: sha2.Settings{
				MaxNumSha2F: tl.PrecompileSha2Blocks,
			},
		}

		// Initialize the Full zkEVM arithmetization
		fullZkEvm = NewZkEVM(settings)

	})

	return fullZkEvm
}
