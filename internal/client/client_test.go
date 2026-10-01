package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      Config
		wantErr string
	}{
		{
			name:    "valid",
			in:      Config{BaseURL: "https://silo.example.org/", APIKey: "k"},
			wantErr: "",
		},
		{
			name:    "empty base url",
			in:      Config{BaseURL: "", APIKey: "k"},
			wantErr: "base_url is empty",
		},
		{
			name:    "missing scheme",
			in:      Config{BaseURL: "silo.example.org", APIKey: "k"},
			wantErr: "must use http or https",
		},
		{
			name:    "includes api path",
			in:      Config{BaseURL: "https://silo.example.org/api/v2", APIKey: "k"},
			wantErr: "without the /api/v2 path",
		},
		{
			name:    "empty api key",
			in:      Config{BaseURL: "https://silo.example.org", APIKey: "  "},
			wantErr: "api_key is empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.in.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestConfigValidateTrimsTrailingSlash(t *testing.T) {
	cfg, err := Config{BaseURL: "https://silo.example.org/", APIKey: "k"}.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://silo.example.org" {
		t.Fatalf("expected trailing slash trimmed, got: %q", cfg.BaseURL)
	}
}

// newTestClient builds a Client against an httptest server. The handler can
// record requests.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cl, err := New(Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	return cl, srv
}

func TestCreateAccessGroupSendsAuthAndReturnsETag(t *testing.T) {
	var gotAuth, gotMethod, gotPath, gotBody string
	cl, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		w.Header().Set("ETag", `"v1"`)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(AccessGroup{ID: "7", Name: "Test"})
	})

	group, etag, err := cl.CreateAccessGroup(context.Background(), &AccessGroupWrite{Name: "Test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.ID != "7" {
		t.Fatalf("expected ID 7, got %q", group.ID)
	}
	if etag != `"v1"` {
		t.Fatalf("expected etag %q, got %q", `"v1"`, etag)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v2/admin/access-groups" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if gotBody != `{"name":"Test"}` {
		t.Fatalf("unexpected request body: %s", gotBody)
	}
}

func TestUpdateAccessGroupSendsIfMatch(t *testing.T) {
	var gotMatch string
	cl, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMatch = r.Header.Get("If-Match")
		w.Header().Set("ETag", `"v2"`)
		_ = json.NewEncoder(w).Encode(AccessGroup{ID: "7", Name: "Test"})
	})

	_, etag, err := cl.UpdateAccessGroup(context.Background(), "7", `"v1"`, &AccessGroupWrite{Name: "Test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMatch != `"v1"` {
		t.Fatalf("expected If-Match %q, got %q", `"v1"`, gotMatch)
	}
	if etag != `"v2"` {
		t.Fatalf("expected etag %q, got %q", `"v2"`, etag)
	}
}

func TestDeleteAccessGroupNotFoundIsSuccess(t *testing.T) {
	cl, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if err := cl.DeleteAccessGroup(context.Background(), "7", `"v1"`); err != nil {
		t.Fatalf("404 should be treated as deleted, got: %v", err)
	}
}

func TestListAllAccessGroupsWalksCursors(t *testing.T) {
	pages := []string{
		`{"items":[{"id":"1","name":"a","member_count":2}],"page":{"has_more":true,"next_cursor":"C2"}}`,
		`{"items":[{"id":"2","name":"b","member_count":0}],"page":{"has_more":false}}`,
	}
	var gotCursors []string
	cl, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCursors = append(gotCursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(pages[len(gotCursors)-1]))
	})

	groups, err := cl.ListAllAccessGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups across pages, got %d", len(groups))
	}
	if groups[0].ID != "1" || groups[1].ID != "2" {
		t.Fatalf("unexpected group order: %s, %s", groups[0].ID, groups[1].ID)
	}
	if groups[0].MemberCount != 2 {
		t.Fatalf("expected member_count 2, got %d", groups[0].MemberCount)
	}
	if len(gotCursors) != 2 || gotCursors[0] != "" || gotCursors[1] != "C2" {
		t.Fatalf("expected cursor walk (\"\", C2), got %v", gotCursors)
	}
}

func TestAPIErrorClassifiers(t *testing.T) {
	makeErr := func(status int) error {
		return newAPIError(http.MethodGet, "/x", &Response{StatusCode: status, Body: []byte(`{"detail":"boom"}`)})
	}
	if !IsNotFound(makeErr(http.StatusNotFound)) {
		t.Fatal("404 should classify as not found")
	}
	if !IsPreconditionFailed(makeErr(http.StatusPreconditionFailed)) {
		t.Fatal("412 should classify as precondition failed")
	}
	if !IsPreconditionRequired(makeErr(http.StatusPreconditionRequired)) {
		t.Fatal("428 should classify as precondition required")
	}
	if IsNotFound(makeErr(http.StatusBadRequest)) {
		t.Fatal("400 should not classify as not found")
	}
	err := makeErr(http.StatusUnprocessableEntity)
	if !IsUnprocessableEntity(err) {
		t.Fatal("422 should classify as unprocessable")
	}
	apiErr, ok := err.(*APIError)
	if !ok || !strings.Contains(apiErr.Error(), "boom") {
		t.Fatalf("problem detail should surface in error, got: %v", err)
	}
}

func TestAccessGroupWriteMarshalsOmittedAndExplicitFields(t *testing.T) {
	// Create body: unset fields are omitted; nullable lists may be nil.
	create := &AccessGroupWrite{Name: "Test"}
	buf, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf) != `{"name":"Test"}` {
		t.Fatalf("unexpected create body: %s", buf)
	}

	// Update body: full desired state, including explicit empty lists.
	empty := []string{}
	description := "d"
	enabled := true
	update := &AccessGroupWrite{
		Name:            "Test",
		Description:     &description,
		LibraryIDs:      &empty,
		DownloadAllowed: &enabled,
	}
	buf, err = json.Marshal(update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := doc["library_ids"]; !ok {
		t.Fatal("explicit empty library_ids must be sent as [], not omitted")
	}
	if got := doc["library_ids"].([]any); len(got) != 0 {
		t.Fatalf("expected empty list, got %v", got)
	}
	if doc["download_allowed"] != true {
		t.Fatalf("expected download_allowed true, got %v", doc["download_allowed"])
	}
}
