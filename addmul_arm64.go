// The MIT License (MIT)
//
// Copyright (C) 2016-2017 Vivint, Inc.
// Copyright (C) 2026 Storj Labs, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build !purego

package infectious

//go:noescape
func addmulNEON(lowhigh *pair, in, out *byte, len int)

func addmul(z, x []byte, y byte) {
	if len(z) == 0 || y == 0 {
		return
	}

	// addmulNEON handles whole 16 byte blocks, the Go loop does the tail.
	done := len(z) &^ 15
	if done > 0 {
		_ = x[done-1]    // the assembly reads x[:done], so check it is in bounds
		z[0] ^= x[0] & 0 // hints to race detector, the assembly is not instrumented
		addmulNEON(&mul_table_pair[y], &x[0], &z[0], done)
		if done == len(z) {
			return
		}
	}

	// hints to the compiler to remove bounds checks
	z = z[done:]
	x = x[done : done+len(z)]

	gf_mul_y := gf_mul_table[y][:]
	for i := range z {
		z[i] ^= gf_mul_y[x[i]]
	}
}

// mulsumMaxLen is the longest row mulsum fuses. mulsumNEON reads 64 bytes
// from every row in turn, and with long rows (it measured 4x slower at 1MiB
// on an M1) that loses to streaming one row at a time through addmul. The
// two measured even between 16KiB and 64KiB.
const mulsumMaxLen = 8 << 10

//go:noescape
func mulsumNEON(tables *[256]pair, coeffs *byte, k int, in *byte, stride int, out *byte, len int)

// mulsum sets out to the sum of coeffs[j] * in[j*len(out):][:len(out)] over
// all j. It replaces clearing out and calling addmul once per row, keeping
// out in registers for all the rows instead of loading and storing it again
// for each of them.
func mulsum(out, in, coeffs []byte) {
	n := len(out)
	if len(coeffs) == 0 || n > mulsumMaxLen {
		mulsumRows(out, in, coeffs)
		return
	}
	in = in[:len(coeffs)*n] // the assembly reads all the rows, so check they are in bounds

	done := n &^ 15
	if done > 0 {
		out[0] ^= in[0] & 0 // hints to race detector, the assembly is not instrumented
		mulsumNEON(&mul_table_pair, &coeffs[0], len(coeffs), &in[0], n, &out[0], done)
	}
	if done < n {
		mulsumTail(out[done:], in[done:], coeffs, n)
	}
}
