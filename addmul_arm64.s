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

//go:build !purego

#include "textflag.h"

// Same split nibble table lookup as addmulAVX2: out ^= low[in&15] ^ high[in>>4].

// func addmulNEON(lowhigh *pair, in, out *byte, len int)
// len must be a multiple of 16.
TEXT ·addmulNEON(SB), NOSPLIT, $0-32
	MOVD lowhigh+0(FP), R0
	MOVD in+8(FP), R1
	MOVD out+16(FP), R2
	MOVD len+24(FP), R3

	VLD1 (R0), [V16.B16, V17.B16] // V16: low, V17: high
	VMOVI $15, V18.B16            // V18: low nibble mask

	CMP $32, R3
	BLT tail16

loop32:
	VLD1.P 32(R1), [V0.B16, V1.B16]
	VLD1   (R2), [V2.B16, V3.B16]

	VAND  V18.B16, V0.B16, V4.B16
	VUSHR $4, V0.B16, V0.B16
	VAND  V18.B16, V1.B16, V5.B16
	VUSHR $4, V1.B16, V1.B16

	VTBL V4.B16, [V16.B16], V4.B16
	VTBL V0.B16, [V17.B16], V0.B16
	VTBL V5.B16, [V16.B16], V5.B16
	VTBL V1.B16, [V17.B16], V1.B16

	VEOR V4.B16, V2.B16, V2.B16
	VEOR V0.B16, V2.B16, V2.B16
	VEOR V5.B16, V3.B16, V3.B16
	VEOR V1.B16, V3.B16, V3.B16

	VST1.P [V2.B16, V3.B16], 32(R2)

	SUB $32, R3
	CMP $32, R3
	BGE loop32

tail16:
	CBZ R3, done

	VLD1 (R1), [V0.B16]
	VLD1 (R2), [V2.B16]

	VAND  V18.B16, V0.B16, V4.B16
	VUSHR $4, V0.B16, V0.B16
	VTBL  V4.B16, [V16.B16], V4.B16
	VTBL  V0.B16, [V17.B16], V0.B16
	VEOR  V4.B16, V2.B16, V2.B16
	VEOR  V0.B16, V2.B16, V2.B16

	VST1 [V2.B16], (R2)

done:
	RET
