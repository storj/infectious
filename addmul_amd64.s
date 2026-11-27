// The MIT License (MIT)
//
// Copyright (C) 2016-2017 Vivint, Inc.
// Copyright (c) 2015 Klaus Post
// Copyright (c) 2015 Backblaze
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

/*
The corresponding C implementations:

void addmul(
	uint8_t * restrict lowhigh,
	uint8_t * restrict in,
	uint8_t * restrict out,
	int n
) {
	for(int i = 0; i < n; i++){
		int value = in[i];
		int low   = value & 15;
		int high  = value >> 4;
		out[i] = out[i] ^ lowhigh[low] ^ lowhigh[high+16];
	}
}

void addmulSSSE3(
	uint8_t * restrict lowhigh,
	uint8_t * restrict in,
	uint8_t * restrict out,
	int n
) {
	int i = 0;

	__m128i lotbl = _mm_loadu_si128((__m128i*)(&lowhigh[0]));
	__m128i hitbl = _mm_loadu_si128((__m128i*)(&lowhigh[16]));

	__m128i lomask = _mm_set1_epi8(0xF);

	#pragma nounroll
	for(i = 0; i < (n/16)*16; i += 16){
		__m128i input8  = _mm_loadu_si128((__m128i*)(&in[i]));
		__m128i output8 = _mm_loadu_si128((__m128i*)(&out[i]));

		__m128i lo8 = _mm_and_si128(lomask, input8);
		__m128i hi8 = _mm_and_si128(lomask, _mm_srli_si128(input8, 4)); // simulate shrli epi8

		output8 = _mm_xor_si128(output8, _mm_shuffle_epi8(lotbl, lo8));
		output8 = _mm_xor_si128(output8, _mm_shuffle_epi8(hitbl, hi8));

		_mm_storeu_si128((__m128i*)(&out[i]), output8);
	}
}

void mulsumAVX2(
	uint8_t * restrict tables, // 256 lowhigh tables, 32 bytes each
	uint8_t * restrict coeffs,
	int k,
	uint8_t * restrict in,
	int stride,
	uint8_t * restrict out,
	int n
) {
	__m256i lomask = _mm256_set1_epi8(0xF);

	// The assembly does four of these 32 byte blocks per pass while it can,
	// so each table load serves 128 bytes.
	#pragma nounroll
	for(int i = 0; i < (n/32)*32; i += 32){
		__m256i sum = _mm256_setzero_si256();

		for(int j = 0; j < k; j++){
			uint8_t *lowhigh = &tables[coeffs[j]*32];
			__m256i lotbl = _mm256_broadcastsi128_si256(_mm_loadu_si128((__m128i*)(&lowhigh[0])));
			__m256i hitbl = _mm256_broadcastsi128_si256(_mm_loadu_si128((__m128i*)(&lowhigh[16])));

			__m256i input = _mm256_loadu_si256((__m256i*)(&in[j*stride + i]));
			__m256i lo    = _mm256_and_si256(lomask, input);
			__m256i hi    = _mm256_and_si256(lomask, _mm256_srli_epi64(input, 4));

			sum = _mm256_xor_si256(sum, _mm256_shuffle_epi8(lotbl, lo));
			sum = _mm256_xor_si256(sum, _mm256_shuffle_epi8(hitbl, hi));
		}

		_mm256_storeu_si256((__m256i*)(&out[i]), sum);
	}
}
*/

#include "textflag.h"
DATA  nybble_mask<>+0x00(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA  nybble_mask<>+0x08(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA  nybble_mask<>+0x10(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA  nybble_mask<>+0x18(SB)/8, $0x0F0F0F0F0F0F0F0F
GLOBL nybble_mask<>(SB), (NOPTR+RODATA), $32

#define LOWHIGH  DI
#define LOW   X8
#define HIGH  X9
#define IN    SI
#define OUT   DX
#define INDEX AX

#define LEN   CX
#define LEN16 R8 // LEN16 = (LEN / 16) * 16

#define LOMASK X7 // LOMASK = repeated 15
// X0-X5 temps

// func addmulSSSE3(lowhigh *[2][16]byte, in, out *byte, len int)
TEXT ·addmulSSSE3(SB), 7, $0
	MOVQ in+8(FP),   IN
	MOVQ out+16(FP), OUT
	MOVQ len+24(FP), LEN

	MOVQ LEN,  LEN16
	ANDQ $-16, LEN16

	JLE start_slow // if LEN16 == 0 { goto done }

	MOVQ lowhigh+0(FP), LOWHIGH
	MOVOU    (LOWHIGH), LOW
	MOVOU  16(LOWHIGH), HIGH

	MOVOU  nybble_mask<>(SB), LOMASK
	XORQ   INDEX, INDEX // INDEX = 0

loop16:
	MOVOU  (IN)(INDEX*1),  X0  // X0 = INPUT[INDEX]
	MOVOU  LOW,  X4            // X4 = copy(LOW)
	MOVOU  (OUT)(INDEX*1), X2  // X2 = OUT[INDEX]
	MOVOU  X0, X1              // X0 = input[index] & 15
	MOVOU  HIGH, X5            // X5 = copy(HIGH)

	PAND   LOMASK, X0
	PSRLQ  $4, X1              // X1 = input[index]
	PSHUFB X0, X4              // X4 = LOW[X0]

	PAND   LOMASK, X1          // X1 = input[index] >> 4
	PSHUFB X1, X5              // X5 = HIGH[X1]
	PXOR   X4, X2              // X2 = OUT[INDEX] ^ X4 ^ X5
	PXOR   X5, X2

	MOVOU X2, 0(OUT)(INDEX*1)

	ADDQ $16,   INDEX
	CMPQ LEN16, INDEX // INDEX < LEN16
	JG loop16

start_slow:
	MOVQ  mul+32(FP), LOWHIGH
	MOVQ LEN16, INDEX
	CMPQ LEN, INDEX
	JLE done

loop1:
	MOVBQZX (IN)(INDEX*1),   R9  // R9  := in[index]
	MOVBQZX (LOWHIGH)(R9*1), R10 // R10 := multiply[R9]
	XORB R10B, (OUT)(INDEX*1)    // out[index] ^= R10
	INCQ INDEX
	CMPQ LEN, INDEX
	JG loop1

done:
	RET

#undef LOWHIGH
#undef LOW
#undef HIGH
#undef IN
#undef OUT
#undef LEN
#undef INDEX
#undef LEN16
#undef LOMASK

// func addmulAVX2(lowhigh *[2][16]byte, in, out *byte, len int)
TEXT ·addmulAVX2(SB), 7, $0
	MOVQ  lowhigh+0(FP), SI // SI: &lowhigh
	MOVOU (SI),   X6        // X6: low
	MOVOU 16(SI), X7        // X7: high

	MOVQ  $15, BX           // BX: low mask
	MOVQ  BX, X5

	MOVQ  len+24(FP), R9 // R9: len(in), len(out)

	VINSERTI128  $1, X6, Y6, Y6 // Y6: low
	VINSERTI128  $1, X7, Y7, Y7 // Y7: high
	VPBROADCASTB X5, Y8         // Y8: lomask (unpacked)

	SHRQ  $5, R9         // len(in) / 32
	MOVQ  out+16(FP), DX // DX: &out
	MOVQ  in+8(FP), SI   // R11: &in
	TESTQ R9, R9
	JZ    done_xor_avx2

loopback_xor_avx2:
	VMOVDQU (SI), Y0   // Y0: input
	VMOVDQU (DX), Y4   // Y4: output
	VPSRLQ  $4, Y0, Y1 // Y1: high input
	VPAND   Y8, Y0, Y0 // Y0: low input
	VPAND   Y8, Y1, Y1 // Y1: high input
	VPSHUFB Y0, Y6, Y2 // Y2: mul low part
	VPSHUFB Y1, Y7, Y3 // Y3: mul high part
	VPXOR   Y3, Y2, Y3 // Y3: result
	VPXOR   Y4, Y3, Y4 // Y4: result
	VMOVDQU Y4, (DX)

	ADDQ $32, SI           // in+=32
	ADDQ $32, DX           // out+=32
	SUBQ $1, R9
	JNZ  loopback_xor_avx2

done_xor_avx2:
	VZEROUPPER
	RET

// func mulsumAVX2(tables *[256]pair, coeffs *byte, k int, in *byte, stride int, out *byte, len int)
// Sets out[:len] to the sum over j < k of coeffs[j] * in[j*stride:][:len].
// k must be positive and len a multiple of 32.
TEXT ·mulsumAVX2(SB), NOSPLIT, $0-56
	MOVQ tables+0(FP), AX
	MOVQ coeffs+8(FP), BX
	MOVQ k+16(FP), CX
	MOVQ in+24(FP), SI
	MOVQ stride+32(FP), R8
	MOVQ out+40(FP), DI
	MOVQ len+48(FP), DX

	VMOVDQU nybble_mask<>(SB), Y14 // Y14: low nibble mask

	CMPQ DX, $128
	JL   check32

loop128:
	VPXOR Y0, Y0, Y0
	VPXOR Y1, Y1, Y1
	VPXOR Y2, Y2, Y2
	VPXOR Y3, Y3, Y3
	MOVQ  SI, R9  // R9: current input row
	MOVQ  BX, R10 // R10: current coefficient
	MOVQ  CX, R11 // R11: rows left

inner128:
	MOVBQZX        (R10), R12
	INCQ           R10
	SHLQ           $5, R12
	ADDQ           AX, R12
	VBROADCASTI128 (R12), Y12   // Y12: low
	VBROADCASTI128 16(R12), Y13 // Y13: high

	VMOVDQU (R9), Y4
	VMOVDQU 32(R9), Y5
	VMOVDQU 64(R9), Y6
	VMOVDQU 96(R9), Y7
	ADDQ    R8, R9

	VPSRLQ $4, Y4, Y8
	VPSRLQ $4, Y5, Y9
	VPSRLQ $4, Y6, Y10
	VPSRLQ $4, Y7, Y11
	VPAND  Y14, Y4, Y4
	VPAND  Y14, Y5, Y5
	VPAND  Y14, Y6, Y6
	VPAND  Y14, Y7, Y7
	VPAND  Y14, Y8, Y8
	VPAND  Y14, Y9, Y9
	VPAND  Y14, Y10, Y10
	VPAND  Y14, Y11, Y11

	VPSHUFB Y4, Y12, Y4
	VPSHUFB Y5, Y12, Y5
	VPSHUFB Y6, Y12, Y6
	VPSHUFB Y7, Y12, Y7
	VPSHUFB Y8, Y13, Y8
	VPSHUFB Y9, Y13, Y9
	VPSHUFB Y10, Y13, Y10
	VPSHUFB Y11, Y13, Y11

	VPXOR Y4, Y0, Y0
	VPXOR Y5, Y1, Y1
	VPXOR Y6, Y2, Y2
	VPXOR Y7, Y3, Y3
	VPXOR Y8, Y0, Y0
	VPXOR Y9, Y1, Y1
	VPXOR Y10, Y2, Y2
	VPXOR Y11, Y3, Y3

	DECQ R11
	JNZ  inner128

	VMOVDQU Y0, (DI)
	VMOVDQU Y1, 32(DI)
	VMOVDQU Y2, 64(DI)
	VMOVDQU Y3, 96(DI)
	ADDQ    $128, SI
	ADDQ    $128, DI
	SUBQ    $128, DX
	CMPQ    DX, $128
	JGE     loop128

check32:
	TESTQ DX, DX
	JZ    mulsumdone

loop32:
	VPXOR Y0, Y0, Y0
	MOVQ  SI, R9
	MOVQ  BX, R10
	MOVQ  CX, R11

inner32:
	MOVBQZX        (R10), R12
	INCQ           R10
	SHLQ           $5, R12
	ADDQ           AX, R12
	VBROADCASTI128 (R12), Y12
	VBROADCASTI128 16(R12), Y13

	VMOVDQU (R9), Y4
	ADDQ    R8, R9

	VPSRLQ  $4, Y4, Y8
	VPAND   Y14, Y4, Y4
	VPAND   Y14, Y8, Y8
	VPSHUFB Y4, Y12, Y4
	VPSHUFB Y8, Y13, Y8
	VPXOR   Y4, Y0, Y0
	VPXOR   Y8, Y0, Y0

	DECQ R11
	JNZ  inner32

	VMOVDQU Y0, (DI)
	ADDQ    $32, SI
	ADDQ    $32, DI
	SUBQ    $32, DX
	JNZ     loop32

mulsumdone:
	VZEROUPPER
	RET
