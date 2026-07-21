package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLockSpec(t *testing.T) {
	var gotMethod string
	var gotBody map[string]bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(Spec{ID: "s1", Locked: true})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.LockSpec(context.Background(), "s1")
	if err != nil {
		t.Fatalf("LockSpec() error = %v", err)
	}
	if gotMethod != http.MethodPatch || !gotBody["locked"] {
		t.Errorf("request = %s body=%v, want PATCH {locked:true}", gotMethod, gotBody)
	}
	if !got.Locked {
		t.Errorf("LockSpec() = %+v, want Locked=true", got)
	}
}

func TestCreateCapture_OmitsEmptyOptionalFields(t *testing.T) {
	var gotBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(Capture{ID: "c1"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.CreateCapture(context.Background(), CaptureInput{ProjectID: "p1", Description: "desc"}); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if _, ok := gotBody["origin_id"]; ok {
		t.Errorf("body has origin_id = %s, want it omitted when empty", gotBody["origin_id"])
	}
	if _, ok := gotBody["tags"]; ok {
		t.Errorf("body has tags = %s, want it omitted when nil", gotBody["tags"])
	}
}

func TestDraftRequest_SerializesAnswers(t *testing.T) {
	var gotBody struct {
		Answers []Answer `json:"answers"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(Request{ID: "r1"})
	}))
	defer srv.Close()

	answers := []Answer{{
		SolutionPathID: "sp1",
		Selections:     []Selection{{Label: "Option A", Custom: false}, {Label: "Something else", Custom: true}},
	}}
	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.DraftRequest(context.Background(), "r1", answers); err != nil {
		t.Fatalf("DraftRequest() error = %v", err)
	}
	if len(gotBody.Answers) != 1 || gotBody.Answers[0].SolutionPathID != "sp1" {
		t.Fatalf("request body = %+v, want one answer for sp1", gotBody.Answers)
	}
	if len(gotBody.Answers[0].Selections) != 2 || !gotBody.Answers[0].Selections[1].Custom {
		t.Errorf("selections = %+v, want second selection marked custom", gotBody.Answers[0].Selections)
	}
}

func TestListSpecs_Success(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_ = json.NewEncoder(w).Encode([]Spec{{ID: "s1"}})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.ListSpecs(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ListSpecs() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("ListSpecs() = %+v, want one spec s1", got)
	}
	if gotPath != "/spec/" || gotQuery != "project_id=p1" {
		t.Errorf("request = %s?%s, want /spec/?project_id=p1", gotPath, gotQuery)
	}
}

func TestListSpecs_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.ListSpecs(context.Background(), "p1"); err == nil {
		t.Error("ListSpecs() error = nil, want the server's 500 to propagate")
	}
}

func TestListRequests_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/request/" || r.URL.RawQuery != "project_id=p1" {
			t.Errorf("request = %s?%s, want /request/?project_id=p1", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]Request{{ID: "r1"}})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.ListRequests(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "r1" {
		t.Errorf("ListRequests() = %+v, want one request r1", got)
	}
}

func TestGetRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/request/r1" {
			t.Errorf("path = %q, want /request/r1", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Request{ID: "r1", Name: "Fix bug"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.GetRequest(context.Background(), "r1")
	if err != nil {
		t.Fatalf("GetRequest() error = %v", err)
	}
	if got.Name != "Fix bug" {
		t.Errorf("GetRequest().Name = %q, want %q", got.Name, "Fix bug")
	}
}

func TestRegenerateRequest_Success(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(Request{ID: "r1"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.RegenerateRequest(context.Background(), "r1", "make it faster")
	if err != nil {
		t.Fatalf("RegenerateRequest() error = %v", err)
	}
	if got.ID != "r1" {
		t.Errorf("RegenerateRequest().ID = %q, want %q", got.ID, "r1")
	}
	if gotMethod != http.MethodPost || gotPath != "/request/r1/regenerate" {
		t.Errorf("request = %s %s, want POST /request/r1/regenerate", gotMethod, gotPath)
	}
	if gotBody["expectation"] != "make it faster" {
		t.Errorf("body.expectation = %q, want %q", gotBody["expectation"], "make it faster")
	}
}

func TestListCaptures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"success", http.StatusOK, false},
		{"server error", http.StatusInternalServerError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/capture/" || r.URL.RawQuery != "project_id=p1" {
					t.Errorf("request = %s?%s, want /capture/?project_id=p1", r.URL.Path, r.URL.RawQuery)
				}
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode([]Capture{{ID: "c1"}})
				}
			}))
			defer srv.Close()

			got, _, err := (&Client{BaseURL: srv.URL}).ListCaptures(context.Background(), "p1")
			if (err != nil) != tt.wantErr {
				t.Fatalf("ListCaptures() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (len(got) != 1 || got[0].ID != "c1") {
				t.Errorf("ListCaptures() = %+v, want one capture c1", got)
			}
		})
	}
}

func TestListSources_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/source" || r.URL.RawQuery != "project_id=p1" {
			t.Errorf("request = %s?%s, want /source?project_id=p1", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]Source{{ID: "src1"}})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.ListSources(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ListSources() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "src1" {
		t.Errorf("ListSources() = %+v, want one source src1", got)
	}
}

func TestListProjects_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.ListProjects(context.Background(), "w1"); err == nil {
		t.Error("ListProjects() error = nil, want the server's 500 to propagate")
	}
}
