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

import (
	"bytes"
	"fmt"
	"math/bits"
	"math/rand"
	"slices"
	"sort"
	"testing"
)

// encodeShares returns all n shares of data, each owning its bytes. Encode
// hands the data shares out as windows into its input, so they have to be
// copied before the caller reuses the input buffer.
func encodeShares(t testing.TB, code *FEC, data []byte) []Share {
	t.Helper()

	shares := make([]Share, code.Total())
	err := code.Encode(data, func(s Share) {
		shares[s.Number] = s.DeepCopy()
	})
	if err != nil {
		t.Fatalf("failed to encode: %s", err)
	}
	return shares
}

// encodeForTest returns all n shares of some deterministic data, plus the
// original data.
func encodeForTest(t testing.TB, code *FEC, block int) (data []byte, shares []Share) {
	t.Helper()

	data = make([]byte, code.Required()*block)
	rand.New(rand.NewSource(42)).Read(data)

	return data, encodeShares(t, code, data)
}

// pickShares returns the named shares, in the order they were named.
func pickShares(shares []Share, nums []int) []Share {
	out := make([]Share, len(nums))
	for i, num := range nums {
		out[i] = shares[num]
	}
	return out
}

// collect runs a rebuild callback and returns the reassembled data, copying
// out of the callback because the buffers it hands over are reused.
func collect(k, block int) (out []byte, store func(Share)) {
	out = make([]byte, k*block)
	return out, func(s Share) {
		copy(out[s.Number*block:(s.Number+1)*block], s.Data)
	}
}

// subsetsOfAtLeast returns every subset of 0..total-1 with at least k members,
// each in ascending order.
func subsetsOfAtLeast(total, k int) [][]int {
	var subsets [][]int
	for mask := 0; mask < 1<<total; mask++ {
		if bits.OnesCount(uint(mask)) < k {
			continue
		}
		nums := make([]int, 0, total)
		for num := range total {
			if mask&(1<<num) != 0 {
				nums = append(nums, num)
			}
		}
		subsets = append(subsets, nums)
	}
	return subsets
}

// checkRebuild runs both (*FEC).Rebuild and the planned rebuild over the same
// shares, requires that the two agree with each other and with want, and
// returns the planned output. label prefixes the failure messages, which name
// the shares and the data themselves, so callers do not have to build a
// description on the happy path -- this runs millions of times.
func checkRebuild(t testing.TB, label string, nums []int, code *FEC, r *Rebuilder, subset []Share, block int, want []byte) []byte {
	k := code.Required()

	wantOut, wantStore := collect(k, block)
	// Rebuild sorts in place, so hand it its own copy.
	if err := code.Rebuild(slices.Clone(subset), wantStore); err != nil {
		t.Fatalf("%sRebuild(%v): %s", label, nums, err)
	}

	gotOut, gotStore := collect(k, block)
	if err := r.Rebuild(subset, gotStore); err != nil {
		t.Fatalf("%splanned Rebuild(%v): %s", label, nums, err)
	}

	if !bytes.Equal(gotOut, wantOut) {
		t.Fatalf("%sshares %v: planned rebuild gave %s, Rebuild gave %s",
			label, nums, brief(gotOut), brief(wantOut))
	}
	if !bytes.Equal(gotOut, want) {
		t.Fatalf("%sshares %v: planned rebuild gave %s, want %s",
			label, nums, brief(gotOut), brief(want))
	}
	return gotOut
}

// brief renders b for a failure message. The large configs carry kilobytes of
// random data, and dumping all of it helps nobody.
func brief(b []byte) string {
	const max = 32
	if len(b) <= max {
		return fmt.Sprintf("%x", b)
	}
	return fmt.Sprintf("%x...(%d bytes)", b[:max], len(b))
}

// TestRebuildPlanMatchesRebuild checks that a planned rebuild reconstructs
// exactly what (*FEC).Rebuild does at production scale, over every share
// subset shape the slot selection can produce: leading data shares, trailing
// parity shares, and random mixtures. TestRebuildPlanExhaustiveSmall covers
// small codes exhaustively; this one covers large ones and the share sizes
// where the vectorized addmul kernel actually runs.
func TestRebuildPlanMatchesRebuild(t *testing.T) {
	const block = 512

	confs := []struct{ required, total int }{
		{2, 4},
		{4, 8},
		{20, 50},
		{29, 80},
	}

	for _, conf := range confs {
		t.Run(fmt.Sprintf("r%dt%d", conf.required, conf.total), func(t *testing.T) {
			code, err := NewFEC(conf.required, conf.total)
			if err != nil {
				t.Fatalf("failed to create new fec code: %s", err)
			}

			data, shares := encodeForTest(t, code, block)
			k := code.Required()

			subsets := [][]int{}

			// the k lowest shares: every slot passes through
			lowest := make([]int, k)
			for i := range lowest {
				lowest[i] = i
			}
			subsets = append(subsets, lowest)

			// the k highest shares: every slot is reconstructed
			highest := make([]int, k)
			for i := range highest {
				highest[i] = conf.total - k + i
			}
			subsets = append(subsets, highest)

			// all k shares supplied plus every extra, exercising the case
			// where the plan has to ignore shares it does not use
			all := make([]int, conf.total)
			for i := range all {
				all[i] = i
			}
			subsets = append(subsets, all)

			// random mixtures
			rng := rand.New(rand.NewSource(7))
			for range 25 {
				perm := rng.Perm(conf.total)[:k]
				sort.Ints(perm)
				subsets = append(subsets, perm)
			}

			for _, nums := range subsets {
				plan, err := code.PlanRebuild(nums)
				if err != nil {
					t.Fatalf("PlanRebuild(%v): %s", nums, err)
				}

				checkRebuild(t, "", nums, code, plan.NewRebuilder(), pickShares(shares, nums), block, data)
			}
		})
	}
}

// TestRebuildPlanExhaustiveSmall pushes every possible two byte input through
// every share subset a small code can produce, checking the planned rebuild
// against (*FEC).Rebuild and against the original data. Two bytes is small
// enough to enumerate outright, so this covers the whole input space rather
// than sampling it. Every conf must divide two bytes evenly; widening the
// input costs a factor of 256 per byte, so this stays at two.
func TestRebuildPlanExhaustiveSmall(t *testing.T) {
	confs := []struct{ required, total int }{
		{1, 3},
		{2, 4},
		{2, 5},
	}

	for _, conf := range confs {
		t.Run(fmt.Sprintf("r%dt%d", conf.required, conf.total), func(t *testing.T) {
			code, err := NewFEC(conf.required, conf.total)
			if err != nil {
				t.Fatalf("failed to create new fec code: %s", err)
			}
			k, block := conf.required, 2/conf.required

			subsets := subsetsOfAtLeast(conf.total, k)

			// One rebuilder per subset, built up front and reused across every
			// input, so the reuse path runs on every iteration.
			rebuilders := make([]*Rebuilder, len(subsets))
			for i, nums := range subsets {
				plan, err := code.PlanRebuild(nums)
				if err != nil {
					t.Fatalf("PlanRebuild(%v): %s", nums, err)
				}
				rebuilders[i] = plan.NewRebuilder()
			}

			data := make([]byte, 2)
			for v := range 1 << 16 {
				data[0], data[1] = byte(v), byte(v>>8)
				shares := encodeShares(t, code, data)

				for i, nums := range subsets {
					checkRebuild(t, "", nums, code, rebuilders[i], pickShares(shares, nums), block, data)
				}
			}
		})
	}
}

// TestRebuildPlanReuse checks that a Rebuilder is reusable across many blocks
// from the same share set, which is the whole point of the API, and that
// reuse does not leak state between calls.
func TestRebuildPlanReuse(t *testing.T) {
	const block = 256
	const required, total = 29, 80

	code, err := NewFEC(required, total)
	if err != nil {
		t.Fatalf("failed to create new fec code: %s", err)
	}

	nums := make([]int, required)
	for i := range nums {
		nums[i] = total - required + i
	}
	plan, err := code.PlanRebuild(nums)
	if err != nil {
		t.Fatalf("PlanRebuild: %s", err)
	}
	rebuilder := plan.NewRebuilder()

	rng := rand.New(rand.NewSource(99))
	for round := range 20 {
		data := make([]byte, required*block)
		rng.Read(data)

		shares := encodeShares(t, code, data)

		out, store := collect(required, block)
		if err := rebuilder.Rebuild(pickShares(shares, nums), store); err != nil {
			t.Fatalf("round %d: %s", round, err)
		}
		if !bytes.Equal(out, data) {
			t.Fatalf("round %d: rebuild does not recover the input", round)
		}
	}
}

func TestRebuildPlanErrors(t *testing.T) {
	const required, total = 4, 8

	code, err := NewFEC(required, total)
	if err != nil {
		t.Fatalf("failed to create new fec code: %s", err)
	}

	if _, err := code.PlanRebuild([]int{0, 1, 2}); err != NotEnoughShares {
		t.Fatalf("too few shares: got %v, want %v", err, NotEnoughShares)
	}
	if _, err := code.PlanRebuild([]int{0, 1, 2, 8}); err == nil {
		t.Fatal("out of range share id: expected an error")
	}
	if _, err := code.PlanRebuild([]int{0, 1, 2, 2}); err == nil {
		t.Fatal("duplicate share id: expected an error")
	}

	plan, err := code.PlanRebuild([]int{4, 5, 6, 7})
	if err != nil {
		t.Fatalf("PlanRebuild: %s", err)
	}
	rebuilder := plan.NewRebuilder()

	// a share the plan needs is absent
	short := []Share{
		{Number: 4, Data: make([]byte, 8)},
		{Number: 5, Data: make([]byte, 8)},
		{Number: 6, Data: make([]byte, 8)},
	}
	if err := rebuilder.Rebuild(short, func(Share) {}); err == nil {
		t.Fatal("missing share: expected an error")
	}

	// shares disagree about their length
	ragged := []Share{
		{Number: 4, Data: make([]byte, 8)},
		{Number: 5, Data: make([]byte, 8)},
		{Number: 6, Data: make([]byte, 4)},
		{Number: 7, Data: make([]byte, 8)},
	}
	if err := rebuilder.Rebuild(ragged, func(Share) {}); err == nil {
		t.Fatal("ragged shares: expected an error")
	}
}

const (
	// fuzzMaxTotal bounds n so every share number is selectable through the
	// uint32 mask the fuzzer supplies.
	fuzzMaxTotal = 32
	// fuzzMaxBlock caps the share size so executions stay fast.
	fuzzMaxBlock = 64
)

// fuzzCode maps two fuzzer bytes onto a code size below fuzzMaxTotal. The seed
// corpus is written against it, so it stays a named function: f.Add takes the
// raw bytes, and this is the only place that says what code they build.
func fuzzCode(kRaw, extraRaw byte) (k, n int) {
	k = 1 + int(kRaw)%(fuzzMaxTotal/2)
	n = k + int(extraRaw)%(fuzzMaxTotal/2)
	return k, n
}

// FuzzRebuildPlan cross-checks the planned rebuild against (*FEC).Rebuild on
// fuzzer chosen codes, share subsets and data, and asserts the properties the
// planned API adds on top: it leaves its inputs alone, and a Rebuilder carries
// no state between calls.
func FuzzRebuildPlan(f *testing.F) {
	f.Add(byte(1), byte(2), uint32(0b0011), []byte("hi"))                          // r2t4, data shares only
	f.Add(byte(1), byte(2), uint32(0b1100), []byte("hi"))                          // r2t4, parity only
	f.Add(byte(0), byte(1), uint32(0b10), []byte("abcd"))                          // r1t2, the parity share
	f.Add(byte(3), byte(4), uint32(0b11110000), []byte("some data to encode!"))    // r4t8, parity only
	f.Add(byte(3), byte(2), ^uint32(0), []byte("0123456789abcdef"))                // r4t6, every share
	f.Add(byte(7), byte(8), uint32(0b1010101010101010), []byte("...............")) // r8t16, alternating
	f.Add(byte(0), byte(15), uint32(1<<15), []byte("x"))                           // r1t16, the highest share

	f.Fuzz(func(t *testing.T, kRaw, extraRaw byte, mask uint32, data []byte) {
		k, n := fuzzCode(kRaw, extraRaw)
		if len(data) < k {
			t.Skip()
		}
		block := min(len(data)/k, fuzzMaxBlock)
		data = data[:k*block]

		// The subset is exactly what the mask selects, so the shares a failing
		// corpus entry used can be read straight off the mask.
		nums := make([]int, 0, n)
		for num := range n {
			if mask&(1<<num) != 0 {
				nums = append(nums, num)
			}
		}
		if len(nums) < k {
			t.Skip()
		}
		// PlanRebuild is documented to accept any order, and descending is the
		// furthest from the ascending order it sorts into.
		slices.Reverse(nums)

		code, err := NewFEC(k, n)
		if err != nil {
			t.Fatalf("NewFEC(%d, %d): %s", k, n, err)
		}
		shares := encodeShares(t, code, data)
		subset := pickShares(shares, nums)

		numsBefore := slices.Clone(nums)
		subsetBefore := make([]Share, len(subset))
		for i := range subset {
			subsetBefore[i] = subset[i].DeepCopy()
		}

		plan, err := code.PlanRebuild(nums)
		if err != nil {
			t.Fatalf("r%dt%d, PlanRebuild(%v): %s", k, n, nums, err)
		}
		rebuilder := plan.NewRebuilder()

		label := fmt.Sprintf("r%dt%d: ", k, n)
		got := checkRebuild(t, label, nums, code, rebuilder, subset, block, data)

		// Neither the share numbers nor the shares themselves may be touched.
		if !slices.Equal(nums, numsBefore) {
			t.Fatalf("%sPlanRebuild modified shareNumbers %v, now %v", label, numsBefore, nums)
		}
		for i := range subsetBefore {
			if subset[i].Number != subsetBefore[i].Number {
				t.Fatalf("%sRebuild reordered shares %v, share %d now at %d",
					label, numsBefore, subset[i].Number, i)
			}
			if !bytes.Equal(subset[i].Data, subsetBefore[i].Data) {
				t.Fatalf("%sRebuild modified the data of share %d", label, subset[i].Number)
			}
		}

		// A second pass through the same Rebuilder must land in the same place.
		checkRebuild(t, label+"reused: ", nums, code, rebuilder, subset, block, got)
	})
}

// BenchmarkRebuildStripes models the access pattern in uplink's stripe
// reader: many small blocks rebuilt from one fixed set of shares. "unplanned"
// is today's (*FEC).Rebuild, which re-inverts the decode matrix per block;
// "planned" inverts once.
func BenchmarkRebuildStripes(b *testing.B) {
	// erasure share sizes seen in production segments
	blocks := []int{256, 1024, 4096}
	confs := []struct{ required, total int }{
		{29, 80},
		{20, 50},
	}

	for _, conf := range confs {
		for _, block := range blocks {
			name := fmt.Sprintf("r%dt%d/share%d", conf.required, conf.total, block)

			code, err := NewFEC(conf.required, conf.total)
			if err != nil {
				b.Fatalf("failed to create new fec code: %s", err)
			}
			_, shares := encodeForTest(b, code, block)

			// worst case for rebuild: only parity shares, so every slot has
			// to be reconstructed
			subset := shares[conf.total-conf.required:]
			nums := make([]int, len(subset))
			for i, s := range subset {
				nums[i] = s.Number
			}

			b.Run(name+"/unplanned", func(b *testing.B) {
				b.SetBytes(int64(block * conf.required))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := code.Rebuild(subset, func(Share) {}); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run(name+"/planned", func(b *testing.B) {
				plan, err := code.PlanRebuild(nums)
				if err != nil {
					b.Fatal(err)
				}
				rebuilder := plan.NewRebuilder()

				b.SetBytes(int64(block * conf.required))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := rebuilder.Rebuild(subset, func(Share) {}); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
