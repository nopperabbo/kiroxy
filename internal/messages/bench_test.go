package messages

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkGateWriter_BufferedWrite(b *testing.B) {
	payload := bytes.Repeat([]byte("data: {\"type\":\"content_block_delta\"}\n\n"), 100)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()

	for b.Loop() {
		rec := httptest.NewRecorder()
		gw := NewGateWriter(rec)
		gw.WriteHeader(http.StatusOK)
		gw.Write(payload)
		gw.Promote()
	}
}

func BenchmarkGateWriter_DirectWrite(b *testing.B) {
	payload := bytes.Repeat([]byte("data: {\"type\":\"content_block_delta\"}\n\n"), 100)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()

	for b.Loop() {
		rec := httptest.NewRecorder()
		gw := NewGateWriter(rec)
		gw.Promote()
		gw.WriteHeader(http.StatusOK)
		gw.Write(payload)
	}
}

func BenchmarkGateWriter_Discard(b *testing.B) {
	payload := bytes.Repeat([]byte("data: {\"type\":\"content_block_delta\"}\n\n"), 50)
	b.ReportAllocs()

	for b.Loop() {
		rec := httptest.NewRecorder()
		gw := NewGateWriter(rec)
		gw.WriteHeader(http.StatusOK)
		gw.Write(payload)
		gw.Discard()
	}
}
