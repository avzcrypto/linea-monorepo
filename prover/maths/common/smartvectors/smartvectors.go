package smartvectors

import (
	"fmt"
	"math/rand"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/zkevm-monorepo/prover/maths/common/vector"
	"github.com/consensys/zkevm-monorepo/prover/maths/field"
	"github.com/consensys/zkevm-monorepo/prover/utils"
)

// SmartVector is an abstraction over vectors of field elements that can be
// optimized for structured vectors. For instance, if we have a vector of
// repeated elements we can use smartvectors.NewConstant(x, n) to represent it.
// This way instead of using n * sizeof(field.Element) memory it will only store
// the element once. Additionally, every operation performed on it will be
// sped up with dedicated algorithms.
//
// There are a few precautions to take when implementing or using smart-vectors
//   - constructing a zero-length smart-vector should be considered illegal. The
//     reason for such a restriction is tha t
//   - although the smart-vectors are not immutable, the user should refrain
//     mutating them after they are created as this may have unintended side
//     effects that are hard to track.
type SmartVector interface {
	// Len returns the length of the SmartVector
	Len() int
	// Get returns an entry of the SmartVector at particular position
	Get(int) field.Element
	// SubVector returns a subvector of the [SmartVector]. It mirrors slice[start:stop]
	SubVector(int, int) SmartVector
	// RotateRight cyclically rotates the SmartVector
	RotateRight(int) SmartVector
	// WriteInSlice writes the SmartVector into a slice. The slice must be just
	// as large as [Len] otherwise the function will panic
	WriteInSlice([]field.Element)
	// Pretty returns a prettified version of the vector, useful for debugging.
	Pretty() string
	// DeepCopy returns a deep-copy of the SmartVector which can be freely
	// mutated without affecting the
	DeepCopy() SmartVector
	// IntoRegVecSaveAlloc converts a smart-vector into a normal vec. The
	// implementation minimizes then number of copies
	IntoRegVecSaveAlloc() []field.Element
}

// AllocateRegular returns a newly allocated smart-vector
func AllocateRegular(n int) SmartVector {
	return NewRegular(make([]field.Element, n))
}

// Copy into a smart-vector, will panic if into is not a regular
// Mainly used as a sugar for refactoring
func Copy(into *SmartVector, x SmartVector) {
	*into = x.DeepCopy()
}

// Rand creates a vector with random entries. Used for testing. Should not be
// used to generate secrets. Not reproducible.
func Rand(n int) SmartVector {
	v := vector.Rand(n)
	return NewRegular(v)
}

// Rand creates a vector with random entries. Used for testing. Should not be
// used to generate secrets. Takes a math.Rand as input for reproducibility
// math
func PseudoRand(rng *rand.Rand, n int) SmartVector {
	return NewRegular(vector.PseudoRand(rng, n))
}

// ForTest returns a witness from a explicit litteral assignement
func ForTest(xs ...int) SmartVector {
	return NewRegular(vector.ForTest(xs...))
}

// IntoRegVec converts a smart-vector into a normal vec. The resulting vector
// is always reallocated and can be safely mutated without side-effects
// on s.
func IntoRegVec(s SmartVector) []field.Element {
	res := make([]field.Element, s.Len())
	s.WriteInSlice(res)
	return res
}

// IntoGnarkAssignment converts a smart-vector into a gnark assignment
func IntoGnarkAssignment(sv SmartVector) []frontend.Variable {
	res := make([]frontend.Variable, sv.Len())
	for i := range res {
		res[i] = sv.Get(i)
	}
	return res
}

// LeftPadded creates a new padded vector (padded on the left)
func LeftPadded(v []field.Element, padding field.Element, targetLen int) SmartVector {

	if len(v) > targetLen {
		utils.Panic("target length %v must be less than %v", len(v), targetLen)
	}

	if len(v) == targetLen {
		return NewRegular(v)
	}

	if len(v) == 0 {
		return NewConstant(padding, targetLen)
	}

	return NewPaddedCircularWindow(v, padding, targetLen-len(v), targetLen)
}

// RightPadded creates a new vector (padded on the right)
func RightPadded(v []field.Element, padding field.Element, targetLen int) SmartVector {

	if len(v) > targetLen {
		utils.Panic("target length %v must be less than %v", len(v), targetLen)
	}

	if len(v) == targetLen {
		return NewRegular(v)
	}

	if len(v) == 0 {
		return NewConstant(padding, targetLen)
	}

	return NewPaddedCircularWindow(v, padding, 0, targetLen)
}

// RightZeroPadded creates a new vector (padded on the right)
func RightZeroPadded(v []field.Element, targetLen int) SmartVector {
	return RightPadded(v, field.Zero(), targetLen)
}

// LeftZeroPadded creates a new vector (padded on the left)
func LeftZeroPadded(v []field.Element, targetLen int) SmartVector {
	return LeftPadded(v, field.Zero(), targetLen)
}

// Density returns the density of a smart-vector. By density we mean the size
// of the concrete underlying vectors. This can be used as a proxi for the
// memory required to store the smart-vector.
func Density(v SmartVector) int {
	switch w := v.(type) {
	case *Constant:
		return 0
	case *PaddedCircularWindow:
		return len(w.window)
	case *Regular:
		return len(*w)
	case *Rotated:
		return len(w.v)
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// Window returns the effective window of the vector,
// if the vector is Padded with zeroes it return the window.
// Namely, the part without zero pads.
func Window(v SmartVector) []field.Element {
	switch w := v.(type) {
	case *Constant:
		return w.IntoRegVecSaveAlloc()
	case *PaddedCircularWindow:
		return w.window
	case *Regular:
		return *w
	case *Rotated:
		return w.IntoRegVecSaveAlloc()
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}