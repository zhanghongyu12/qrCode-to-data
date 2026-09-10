package send

import (
	"testing"

	"qrcd/internal/payload"
)

func TestTotalSymbolsOverall(t *testing.T) {
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 251)
	}
	load := &payload.Load{Data: data, Name: "big.bin", PayloadType: "file"}
	streams, err := BuildSessionStreams(load, Options{Version: 15, ECC: "L", Redundancy: 0.25, BlockSize: 1024, MaxSymbol: 350})
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) <= 1 {
		t.Fatalf("expected multi-part, got %d", len(streams))
	}
	sum := 0
	for _, st := range streams {
		sum += st.TotalData()
	}
	for i, st := range streams {
		if st.Meta().TotalSymbolsOverall != sum {
			t.Fatalf("part %d TotalSymbolsOverall=%d want %d", i, st.Meta().TotalSymbolsOverall, sum)
		}
	}
	t.Logf("parts=%d sum=%d per-part=%d", len(streams), sum, streams[0].TotalData())
}
