package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"local/kiroxy/internal/doctor"
)

type stubTools struct {
	rep *doctor.Report
}

func (s *stubTools) Doctor(ctx context.Context) *doctor.Report { return s.rep }

func TestTools_DoctorDisabled404(t *testing.T) {
	s := New(Options{})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/tools/doctor")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res.StatusCode)
	}
}

func TestTools_DoctorOK(t *testing.T) {
	rep := &doctor.Report{
		OK:      true,
		Results: []doctor.Result{{Name: "runtime", Status: doctor.StatusOK}},
	}
	s := New(Options{ToolsProvider: &stubTools{rep: rep}})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/tools/doctor")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var got doctor.Report
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || len(got.Results) != 1 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestTools_DoctorPOSTAlsoWorks(t *testing.T) {
	rep := &doctor.Report{OK: false, Results: []doctor.Result{{Name: "vault", Status: doctor.StatusError}}}
	s := New(Options{ToolsProvider: &stubTools{rep: rep}})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/dashboard/api/tools/doctor", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var got doctor.Report
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.OK {
		t.Fatalf("expected OK=false")
	}
}

func TestTools_DoctorNilReport500(t *testing.T) {
	s := New(Options{ToolsProvider: &stubTools{rep: nil}})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/tools/doctor")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 500 {
		t.Fatalf("want 500, got %d", res.StatusCode)
	}
}
