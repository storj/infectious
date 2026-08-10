// The MIT License (MIT)
//
// Copyright (C) 2016-2017 Vivint, Inc.
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

import "testing"

func TestGFValPow(t *testing.T) {
	for b := 0; b < 256; b++ {
		naive := gfVal(1)
		for val := 0; val < 600; val++ {
			if got := gfVal(b).pow(val); got != naive {
				t.Fatalf("%d.pow(%d) = %02x, want %02x", b, val, got, naive)
			}
			naive = naive.mul(gfVal(b))
		}
	}

	// negative exponents are taken in the multiplicative group, so
	// b.pow(-val) is inv(b).pow(val).
	for b := 1; b < 256; b++ {
		inv, err := gfVal(b).inv()
		if err != nil {
			t.Fatal(err)
		}
		naive := gfVal(1)
		for val := 0; val < 600; val++ {
			if got := gfVal(b).pow(-val); got != naive {
				t.Fatalf("%d.pow(%d) = %02x, want %02x", b, -val, got, naive)
			}
			naive = naive.mul(inv)
		}
	}
	if got := gfVal(0).pow(-1); got != 0 {
		t.Fatalf("0.pow(-1) = %02x, want 0", got)
	}
}

func TestGFPolyEval(t *testing.T) {
	for _, p := range []gfPoly{nil, {0x35}, {0x01, 0x00, 0xac, 0x5e}, {0xfe, 0x35, 0x02, 0x01, 0x00}} {
		for x := 0; x < 256; x++ {
			naive := gfConst(0)
			for i := 0; i <= p.deg(); i++ {
				naive = naive.add(p.index(i).mul(gfVal(x).pow(i)))
			}
			if got := p.eval(gfVal(x)); got != naive {
				t.Fatalf("%02x.eval(%02x) = %02x, want %02x", p, x, got, naive)
			}
		}
	}
}

func TestGFPolyDiv(t *testing.T) {
	q := gfPoly{
		0x5e, 0x60, 0x8c, 0x3d, 0xc6, 0x8e, 0x7e, 0xa5, 0x2c, 0xa4, 0x04, 0x8a,
		0x2b, 0xc2, 0x36, 0x0f, 0xfc, 0x3f, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	e := gfPoly{
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	_, _, err := q.div(e)
	if err != nil {
		t.Fatal(err)
	}
}
