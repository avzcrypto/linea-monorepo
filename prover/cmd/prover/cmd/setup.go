package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	blob_v0 "github.com/consensys/zkevm-monorepo/prover/lib/compressor/blob/v0"
	blob_v1 "github.com/consensys/zkevm-monorepo/prover/lib/compressor/blob/v1"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/zkevm-monorepo/prover/circuits"
	"github.com/consensys/zkevm-monorepo/prover/circuits/aggregation"
	v0 "github.com/consensys/zkevm-monorepo/prover/circuits/blobdecompression/v0"
	v1 "github.com/consensys/zkevm-monorepo/prover/circuits/blobdecompression/v1"
	"github.com/consensys/zkevm-monorepo/prover/circuits/dummy"
	"github.com/consensys/zkevm-monorepo/prover/circuits/emulation"
	"github.com/consensys/zkevm-monorepo/prover/circuits/execution"
	"github.com/consensys/zkevm-monorepo/prover/config"
	"github.com/consensys/zkevm-monorepo/prover/utils"
	"github.com/consensys/zkevm-monorepo/prover/zkevm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/consensys/gnark/backend/plonk"
)

var (
	fForce     bool
	fCircuits  string
	fDictPath  string
	fAssetsDir string
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "pre compute assets for Linea circuits",
	RunE:  cmdSetup,
}

var allCircuits = []string{
	string(circuits.ExecutionCircuitID),
	string(circuits.ExecutionLargeCircuitID),
	string(circuits.BlobDecompressionV0CircuitID),
	string(circuits.BlobDecompressionV1CircuitID),
	string(circuits.AggregationCircuitID),
	string(circuits.EmulationCircuitID),
	string(circuits.EmulationDummyCircuitID), // we want to generate Verifier.sol for this one
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVar(&fForce, "force", false, "overwrites existing files")
	setupCmd.Flags().StringVar(&fCircuits, "circuits", strings.Join(allCircuits, ","), "comma separated list of circuits to setup")
	setupCmd.Flags().StringVar(&fDictPath, "dict", "", "path to the dictionary file used in blob (de)compression")
	setupCmd.Flags().StringVar(&fAssetsDir, "assets-dir", "", "path to the directory where the assets are stored (override conf)")

	viper.BindPFlag("assets_dir", setupCmd.Flags().Lookup("assets-dir"))
}

func cmdSetup(cmd *cobra.Command, args []string) error {
	// read config
	cfg, err := config.NewConfigFromFile(fConfigFile)
	if err != nil {
		return fmt.Errorf("%s failed to read config file: %w", cmd.Name(), err)
	}

	if fDictPath != "" {
		// fail early if the dictionary file is not found but was specified.
		if _, err := os.Stat(fDictPath); err != nil {
			return fmt.Errorf("%s dictionary file not found: %w", cmd.Name(), err)
		}
	}

	// parse inCircuits
	inCircuits := make(map[circuits.CircuitID]bool)
	for _, c := range allCircuits {
		inCircuits[circuits.CircuitID(c)] = false
	}
	_inCircuits := strings.Split(fCircuits, ",")
	for _, c := range _inCircuits {
		if _, ok := inCircuits[circuits.CircuitID(c)]; !ok {
			return fmt.Errorf("%s unknown circuit: %s", cmd.Name(), c)
		}
		inCircuits[circuits.CircuitID(c)] = true
	}

	// create assets dir if needed (example; efs://prover-assets/v0.1.0/)
	os.MkdirAll(filepath.Join(cfg.AssetsDir, cfg.Version), 0755)

	// srs provider
	var srsProvider circuits.SRSProvider
	srsProvider, err = circuits.NewSRSStore(cfg.PathForSRS())
	if err != nil {
		return fmt.Errorf("%s failed to create SRS provider: %w", cmd.Name(), err)
	}

	// for each circuit, we start by compiling the circuit
	// the we do a shashum and compare against the one in the manifest.json
	for c, setup := range inCircuits {
		if !setup {
			// we skip aggregation in this first loop since the setup is more complex
			continue
		}
		logrus.Infof("setting up %s", c)

		var builder circuits.Builder
		var dict []byte
		extraFlags := make(map[string]any)

		// let's compile the circuit.
		switch c {
		case circuits.ExecutionCircuitID, circuits.ExecutionLargeCircuitID:
			limits := cfg.TracesLimits
			if c == circuits.ExecutionLargeCircuitID {
				limits = cfg.TracesLimitsLarge
			}
			extraFlags["cfg_checksum"] = limits.Checksum() + cfg.Execution.Features.Checksum()
			zkEvm := zkevm.FullZkEvm(&cfg.Execution.Features, &limits)
			builder = execution.NewBuilder(zkEvm)
		case circuits.BlobDecompressionV0CircuitID, circuits.BlobDecompressionV1CircuitID:
			dict, err = os.ReadFile(fDictPath)
			if err != nil {
				return fmt.Errorf("%s failed to read dictionary file: %w", cmd.Name(), err)
			}

			if c == circuits.BlobDecompressionV0CircuitID {
				extraFlags["maxUsableBytes"] = blob_v0.MaxUsableBytes
				extraFlags["maxUncompressedBytes"] = blob_v0.MaxUncompressedBytes
				builder = v0.NewBuilder(dict)
			} else if c == circuits.BlobDecompressionV1CircuitID {
				extraFlags["maxUsableBytes"] = blob_v1.MaxUsableBytes
				extraFlags["maxUncompressedBytes"] = blob_v1.MaxUncompressedBytes
				builder = v1.NewBuilder(len(dict))
			}
		case circuits.EmulationDummyCircuitID:
			// we can get the Verifier.sol from there.
			builder = dummy.NewBuilder(circuits.MockCircuitIDEmulation, ecc.BN254.ScalarField())
		default:
			continue // dummy, aggregation or emulation circuits are handled later
		}

		if err := updateSetup(cmd.Context(), cfg, srsProvider, c, builder, extraFlags); err != nil {
			return err
		}
		if dict != nil {
			// we save the dictionary to disk
			dictPath := filepath.Join(cfg.PathForSetup(string(c)), config.DictionaryFileName)
			if err := os.WriteFile(dictPath, dict, 0600); err != nil {
				return fmt.Errorf("%s failed to write dictionary file: %w", cmd.Name(), err)
			}
		}

	}

	if !(inCircuits[circuits.AggregationCircuitID] || inCircuits[circuits.EmulationCircuitID]) {
		// we are done
		return nil
	}

	// first, we need to collect the verifying keys
	var allowedVkForAggregation []plonk.VerifyingKey
	for _, allowedInput := range cfg.Aggregation.AllowedInputs {
		// first if it's a dummy circuit, we just run the setup here, we don't need to persist it.
		if isDummyCircuit(allowedInput) {
			var curveID ecc.ID
			var mockID circuits.MockCircuitID
			switch allowedInput {
			case string(circuits.ExecutionDummyCircuitID):
				curveID = ecc.BLS12_377
				mockID = circuits.MockCircuitIDExecution
			case string(circuits.BlobDecompressionDummyCircuitID):
				curveID = ecc.BLS12_377
				mockID = circuits.MockCircuitIDDecompression
			case string(circuits.EmulationDummyCircuitID):
				curveID = ecc.BN254
				mockID = circuits.MockCircuitIDEmulation
			default:
				return fmt.Errorf("unknown dummy circuit: %s", allowedInput)
			}

			vk, err := getDummyCircuitVK(cmd.Context(), cfg, srsProvider, circuits.CircuitID(allowedInput), dummy.NewBuilder(mockID, curveID.ScalarField()))
			if err != nil {
				return err
			}
			allowedVkForAggregation = append(allowedVkForAggregation, vk)
			continue
		}

		// derive the asset paths
		setupPath := cfg.PathForSetup(allowedInput)
		vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)
		vk := plonk.NewVerifyingKey(ecc.BLS12_377)
		if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
			return fmt.Errorf("%s failed to read verifying key for circuit %s: %w", cmd.Name(), allowedInput, err)
		}

		allowedVkForAggregation = append(allowedVkForAggregation, vk)
	}

	// we need to compute the digest of the verifying keys & store them in the manifest
	// for the aggregation circuits to be able to check compatibility at run time with the proofs
	allowedVkForAggregationDigests := listOfCheckum(allowedVkForAggregation)
	extraFlagsForAggregationCircuit := map[string]any{
		"allowedVkForAggregationDigests": allowedVkForAggregationDigests,
	}

	// now for each aggregation circuit, we update the setup if needed, and collect the verifying keys
	var allowedVkForEmulation []plonk.VerifyingKey
	for _, numProofs := range cfg.Aggregation.NumProofs {
		c := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), numProofs))
		logrus.Infof("setting up %s (numProofs=%d)", c, numProofs)

		builder := aggregation.NewBuilder(numProofs, allowedVkForAggregation)
		if err := updateSetup(cmd.Context(), cfg, srsProvider, c, builder, extraFlagsForAggregationCircuit); err != nil {
			return err
		}

		// read the verifying key
		setupPath := cfg.PathForSetup(string(c))
		vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)
		vk := plonk.NewVerifyingKey(ecc.BW6_761)
		if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
			return fmt.Errorf("%s failed to read verifying key for circuit %s: %w", cmd.Name(), c, err)
		}

		allowedVkForEmulation = append(allowedVkForEmulation, vk)
	}

	// now we can update the final (emulation) circuit
	c := circuits.EmulationCircuitID
	logrus.Infof("setting up %s", c)
	builder := emulation.NewBuilder(allowedVkForEmulation)
	return updateSetup(cmd.Context(), cfg, srsProvider, c, builder, nil)

}

func isDummyCircuit(cID string) bool {
	switch circuits.CircuitID(cID) {
	case circuits.ExecutionDummyCircuitID, circuits.BlobDecompressionDummyCircuitID, circuits.EmulationDummyCircuitID:
		return true
	}
	return false

}

func getDummyCircuitVK(ctx context.Context, cfg *config.Config, srsProvider circuits.SRSProvider, circuit circuits.CircuitID, builder circuits.Builder) (plonk.VerifyingKey, error) {
	// compile the circuit
	logrus.Infof("compiling %s", circuit)
	ccs, err := builder.Compile()
	if err != nil {
		return nil, fmt.Errorf("failed to compile circuit %s: %w", circuit, err)
	}
	setup, err := circuits.MakeSetup(ctx, circuit, ccs, srsProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to setup circuit %s: %w", circuit, err)
	}

	return setup.VerifyingKey, nil
}

// updateSetup runs the setup for the given circuit if needed.
// it first compiles the circuit, then checks if the files already exist,
// and if so, if the checksums match.
// if the files already exist and the checksums match, it skips the setup.
// else it does the setup and writes the assets to disk.
func updateSetup(ctx context.Context, cfg *config.Config, srsProvider circuits.SRSProvider, circuit circuits.CircuitID, builder circuits.Builder, extraFlags map[string]any) error {
	if extraFlags == nil {
		extraFlags = make(map[string]any)
	}

	// compile the circuit
	logrus.Infof("compiling %s", circuit)
	ccs, err := builder.Compile()
	if err != nil {
		return fmt.Errorf("failed to compile circuit %s: %w", circuit, err)
	}

	// derive the asset paths
	setupPath := cfg.PathForSetup(string(circuit))
	manifestPath := filepath.Join(setupPath, config.ManifestFileName)

	if !fForce {
		// we may want to skip setup if the files already exist
		// and the checksums match
		// read manifest if already exists
		if manifest, err := circuits.ReadSetupManifest(manifestPath); err == nil {
			circuitDigest, err := circuits.CircuitDigest(ccs)
			if err != nil {
				return fmt.Errorf("failed to compute circuit digest for circuit %s: %w", circuit, err)
			}

			if manifest.Checksums.Circuit == circuitDigest {
				logrus.Infof("skipping %s (already setup)", circuit)
				return nil
			}
		}
	}

	// run the actual setup
	logrus.Infof("plonk setup for %s", circuit)
	setup, err := circuits.MakeSetup(ctx, circuit, ccs, srsProvider, extraFlags)
	if err != nil {
		return fmt.Errorf("failed to setup circuit %s: %w", circuit, err)
	}

	logrus.Infof("writing assets for %s", circuit)
	return setup.WriteTo(setupPath)
}

// listOfCheckum Computes a list of SHA256 checksums for a list of assets, the result is given
// in hexstring.
func listOfCheckum[T io.WriterTo](assets []T) []string {
	res := make([]string, len(assets))
	for i := range assets {
		h := sha256.New()
		_, err := assets[i].WriteTo(h)
		if err != nil {
			// It is unexpected that writing in a hasher could possibly fail.
			panic(err)
		}
		digest := h.Sum(nil)
		res[i] = utils.HexEncodeToString(digest)
	}
	return res
}