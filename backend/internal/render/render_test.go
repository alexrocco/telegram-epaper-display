package render

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/alexxrocco/telegram-epaper-display/backend/internal/store"
)

func sampleView() View {
	base := time.Date(2026, 6, 22, 14, 5, 0, 0, time.UTC)
	return View{
		ChannelTitle: "Oia o Trem",
		UpdatedAt:    base.Add(2 * time.Minute),
		Messages: []store.Message{
			{ID: 3, Date: base.Add(time.Minute), Author: "João",
				Text: "Atenção: o próximo trem com destino à estação central está com atraso de 8 minutos."},
			{ID: 2, Date: base,
				Text: "Boas notícias! A manutenção na linha verde foi concluída e a operação está normalizada."},
			{ID: 1, Date: base.Add(-30 * time.Minute),
				Text: "Plataforma 2 temporariamente fechada para limpeza."},
		},
	}
}

func TestPackBufferSize(t *testing.T) {
	buf := Pack(Render(sampleView()))
	if len(buf) != BufferSize {
		t.Fatalf("buffer size = %d, want %d", len(buf), BufferSize)
	}
}

func TestPackLevelsInRange(t *testing.T) {
	buf := Pack(Render(sampleView()))
	// Every 2-bit nibble must be a valid level 0..3 (trivially true for bytes,
	// but assert the buffer isn't uniformly blank/white).
	allWhite := true
	for _, b := range buf {
		if b != 0xFF {
			allWhite = false
			break
		}
	}
	if allWhite {
		t.Fatal("rendered buffer is entirely white; nothing was drawn")
	}
}

// TestDumpPreview writes a PNG preview for visual inspection when DUMP_PREVIEW
// is set: DUMP_PREVIEW=/tmp/preview.png go test ./internal/render/ -run Preview
func TestDumpPreview(t *testing.T) {
	path := os.Getenv("DUMP_PREVIEW")
	if path == "" {
		t.Skip("set DUMP_PREVIEW=/path/to.png to dump a preview")
	}
	img := PreviewImage(Render(sampleView()))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote preview to %s", path)
}
