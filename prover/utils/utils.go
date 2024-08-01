package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
)

/*
	All * standard * functions that we manually implement
*/

// Return true if n is a power of two
func IsPowerOfTwo[T ~int](n T) bool {
	return n&(n-1) == 0 && n > 0
}

// DivCeil for int a, b
func DivCeil(a, b int) int {
	res := a / b
	if b*res < a {
		return res + 1
	}
	return res
}

// DivExact for int a, b. Panics if b does not divide a exactly.
func DivExact(a, b int) int {
	res := a / b
	if res*b != a {
		panic("inexact division")
	}
	return res
}

// Iterates the function on all the given arguments and return an error
// if one is not equal to the first one. Panics if given an empty array.
func AllReturnEqual[T, U any](fs func(T) U, args []T) (U, error) {

	if len(args) < 1 {
		Panic("Empty list of slice")
	}

	first := fs(args[0])

	for _, arg := range args[1:] {
		curr := fs(arg)
		if !reflect.DeepEqual(first, curr) {
			return first, fmt.Errorf("mismatch between %v %v, got %v != %v",
				args[0], arg, first, curr,
			)
		}
	}

	return first, nil
}

/*
NextPowerOfTwo returns the next power of two for the given number.
It returns the number itself if it's a power of two. As an edge case,
zero returns zero.

Taken from :
https://github.com/protolambda/zrnt/blob/v0.13.2/eth2/util/math/math_util.go#L58
The function panics if the input is more than  2**62 as this causes overflow
*/
func NextPowerOfTwo[T ~int](in T) T {
	if in > 1<<62 {
		panic("Input is too large")
	}
	v := in
	v--
	v |= v >> (1 << 0)
	v |= v >> (1 << 1)
	v |= v >> (1 << 2)
	v |= v >> (1 << 3)
	v |= v >> (1 << 4)
	v |= v >> (1 << 5)
	v++
	return v
}

// PositiveMod returns the positive modulus
func PositiveMod[T ~int](a, n T) T {
	res := a % n
	if res < 0 {
		return res + n
	}
	return res
}

// Join joins a set of slices by appending them into a new array. It can also
// be used to flatten a double array.
func Join[T any](ts ...[]T) []T {
	res := []T{}
	for _, t := range ts {
		res = append(res, t...)
	}
	return res
}

// Log2Floor computes the floored value of Log2
func Log2Floor(a int) int {
	res := 0
	for i := a; i > 1; i = i >> 1 {
		res++
	}
	return res
}

// Log2Ceil computes the ceiled value of Log2
func Log2Ceil(a int) int {
	floor := Log2Floor(a)
	if a != 1<<floor {
		floor++
	}
	return floor
}

// GCD calculates GCD of a and b by Euclidian algorithm.
func GCD[T ~int](a, b T) T {
	for a != b {
		if a > b {
			a -= b
		} else {
			b -= a
		}
	}

	return a
}

// Returns a SHA256 checksum of the given asset.
// TODO @gbotrel merge with Digest
// Sha2SumHexOf returns a SHA256 checksum of the given asset.
func Sha2SumHexOf(w io.WriterTo) string {
	hasher := sha256.New()
	w.WriteTo(hasher)
	res := hasher.Sum(nil)
	return HexEncodeToString(res)
}

// Digest computes the SHA256 Digest of the contents of file and prepends a "0x"
// byte to it. Callers are responsible for closing the file. The reliance on
// SHA256 is motivated by the fact that we use the sum checksum for the verifier
// key to identify which verifier contract to use.
func Digest(src io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, src); err != nil {
		return "", fmt.Errorf("copy into hasher: %w", err)
	}

	return "0x" + hex.EncodeToString(h.Sum(nil)), nil
}

// RightPad copies `s` and returns a vector padded up the length `n` using
// `padWith` as a filling value. The function panics if len(s) < n and returns
// a copy of s if len(s) == n.
func RightPad[T any](s []T, n int, padWith T) []T {
	res := append(make([]T, 0, n), s...)
	for len(res) < n {
		res = append(res, padWith)
	}
	return res
}

// RepeatSlice returns the concatenation of `s` with itself `n` times
func RepeatSlice[T any](s []T, n int) []T {
	res := make([]T, 0, n*len(s))
	for i := 0; i < n; i++ {
		res = append(res, s...)
	}
	return res
}