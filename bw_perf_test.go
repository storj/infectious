package infectious

import (
	"testing"
)

// BenchmarkCorrectCorruptedShare corrupts one entire share so that
// berlekampWelch runs for every byte column, isolating its cost from the
// syndrome scan.
func BenchmarkCorrectCorruptedShare(b *testing.B) {
	const block = 64
	const total, required = 40, 20

	code, err := NewFEC(required, total)
	if err != nil {
		b.Fatal(err)
	}

	data := make([]byte, required*block)
	for i := range data {
		data[i] = byte(i)
	}
	origin := make([]Share, total)
	err = code.Encode(data, func(s Share) {
		origin[s.Number] = s.DeepCopy()
	})
	if err != nil {
		b.Fatal(err)
	}

	shares := make([]Share, total)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for j := range origin {
			shares[j] = origin[j].DeepCopy()
		}
		for j := range shares[0].Data {
			shares[0].Data[j] ^= 0x5a
		}
		b.StartTimer()

		if err := code.Correct(shares); err != nil {
			b.Fatal(err)
		}
	}
}
