// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by consensys/gnark-crypto DO NOT EDIT

package fft

import (
	"github.com/consensys/zkevm-monorepo/prover/maths/field"
)

// Domain with a power of 2 cardinality
// compute a field element of order 2x and store it in FinerGenerator
// all other values can be derived from x, GeneratorSqrt
type Domain struct {
	Cardinality            uint64
	CardinalityInv         field.Element
	Generator              field.Element
	GeneratorInv           field.Element
	FrMultiplicativeGen    field.Element // generator of Fr*
	FrMultiplicativeGenInv field.Element

	// the following slices are not serialized and are (re)computed through domain.preComputeTwiddles()

	// Twiddles factor for the FFT using Generator for each stage of the recursive FFT
	Twiddles [][]field.Element

	// Twiddles factor for the FFT using GeneratorInv for each stage of the recursive FFT
	TwiddlesInv [][]field.Element

	// we precompute these mostly to avoid the memory intensive bit reverse permutation in the groth16.Prover

	// CosetTable u*<1,g,..,g^(n-1)>
	CosetTable         []field.Element
	CosetTableReversed []field.Element // optional, this is computed on demand at the creation of the domain

	// CosetTable[i][j] = domain.Generator(i-th)SqrtInv ^ j
	CosetTableInv         []field.Element
	CosetTableInvReversed []field.Element // optional, this is computed on demand at the creation of the domain
}