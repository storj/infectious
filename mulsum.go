// The MIT License (MIT)
//
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

package infectious

// mulsumRows is mulsum done one row at a time through addmul. The assembly
// mulsum falls back to it for long rows.
func mulsumRows(out, in, coeffs []byte) {
	clear(out)
	n := len(out)
	for j, c := range coeffs {
		addmul(out, in[j*n:(j+1)*n], c)
	}
}

// mulsumTail is mulsum for the few bytes left after the assembly, where
// calling addmul once per row costs more than the bytes it handles. Row j
// starts at in[j*stride:].
func mulsumTail(out, in, coeffs []byte, stride int) {
	clear(out)
	for j, c := range coeffs {
		mul := &gf_mul_table[c]
		row := in[j*stride:][:len(out)]
		for i := range out {
			out[i] ^= mul[row[i]]
		}
	}
}
