package messages

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func FuzzGateWriter_Write(f *testing.F) {
	f.Add([]byte("data: {\"type\":\"content_block_delta\"}\n\n"))
	f.Add([]byte(""))
	f.Add(bytes.Repeat([]byte("x"), 65536))

	f.Fuzz(func(t *testing.T, data []byte) {
		rec := httptest.NewRecorder()
		gw := NewGateWriter(rec)
		gw.WriteHeader(http.StatusOK)
		n, err := gw.Write(data)
		if err != nil {
			t.Skip()
		}
		if n != len(data) {
			t.Errorf("wrote %d, want %d", n, len(data))
		}
		gw.Promote()
		if rec.Body.Len() != len(data) {
			t.Errorf("promoted body %d, want %d", rec.Body.Len(), len(data))
		}
	})
}
