package apiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorder captures everything the client sent so the test can assert on
// method, path, content-type, body, and the auth cookie in one place. The
// handler replies with whatever bodyToReturn / statusToReturn are set to at
// call time.
type recorder struct {
	mu sync.Mutex

	method      string
	path        string
	rawQuery    string
	contentType string
	authCookie  string
	body        []byte

	statusToReturn int
	bodyToReturn   string
}

func (r *recorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.method = req.Method
		r.path = req.URL.Path
		r.rawQuery = req.URL.RawQuery
		r.contentType = req.Header.Get("Content-Type")
		if c, err := req.Cookie("auth"); err == nil {
			r.authCookie = c.Value
		}
		r.body, _ = io.ReadAll(req.Body)

		status := r.statusToReturn
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(r.bodyToReturn))
	})
}

// newTestServer wires up a recorder + httptest.Server + Client triple and
// returns them ready to drive. Caller closes the server via t.Cleanup.
func newTestServer(t *testing.T) (*recorder, *Client) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler())
	t.Cleanup(srv.Close)
	c := New(srv.URL, "test-token")
	return rec, c
}

// ---- per-method wire-protocol assertions ---------------------------------
//
// One test case per Client method, table-driven. We check that the request
// goes to the right URL with the right verb and the right shape so server-
// side route renames trip a loud failure here.

func TestClient_Endpoints(t *testing.T) {
	type call struct {
		name        string
		bodyReturn  string
		wantMethod  string
		wantPath    string
		wantReqBody string // empty means "no body expected"
		run         func(c *Client) error
	}

	cases := []call{
		{
			name:       "ListOrganizations",
			bodyReturn: `{"organizations":[{"id":"o1","name":"Acme"}]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/",
			run: func(c *Client) error {
				orgs, err := c.ListOrganizations(context.Background())
				if err != nil {
					return err
				}
				if len(orgs) != 1 || orgs[0].ID != "o1" {
					t.Errorf("ListOrganizations result = %+v", orgs)
				}
				return nil
			},
		},
		{
			name:       "ListApps",
			bodyReturn: `{"organization":{"id":"o1","name":"Acme"},"applications":[{"id":"a1","name":"web"}]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/a",
			run: func(c *Client) error {
				apps, err := c.ListApps(context.Background(), "o1")
				if err != nil {
					return err
				}
				if len(apps) != 1 || apps[0].ID != "a1" {
					t.Errorf("ListApps result = %+v", apps)
				}
				return nil
			},
		},
		{
			name:       "GetApp",
			bodyReturn: `{"id":"a1","name":"web"}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/a/a1",
			run: func(c *Client) error {
				app, err := c.GetApp(context.Background(), "o1", "a1")
				if err != nil {
					return err
				}
				if app.ID != "a1" {
					t.Errorf("GetApp result = %+v", app)
				}
				return nil
			},
		},
		{
			name:        "CreateApp",
			bodyReturn:  `{"id":"a1","name":"web"}`,
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/o1/a",
			wantReqBody: `{"name":"web","description":"a service"}`,
			run: func(c *Client) error {
				_, err := c.CreateApp(context.Background(), "o1", "web", "a service")
				return err
			},
		},
		{
			name:        "CreateEnvironment",
			bodyReturn:  `{}`,
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/o1/a/a1/e",
			wantReqBody: `{"name":"production"}`,
			run: func(c *Client) error {
				return c.CreateEnvironment(context.Background(), "o1", "a1", "production")
			},
		},
		{
			name:       "DeleteEnvironment",
			bodyReturn: ``,
			wantMethod: http.MethodDelete,
			wantPath:   "/organizations/o1/a/a1/e/staging",
			run: func(c *Client) error {
				return c.DeleteEnvironment(context.Background(), "o1", "a1", "staging")
			},
		},
		{
			name:       "ListComponentDefinitions",
			bodyReturn: `{"definitions":[{"id":"d1","name":"web","type":"web-service"}]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/component-definitions",
			run: func(c *Client) error {
				defs, err := c.ListComponentDefinitions(context.Background(), "o1")
				if err != nil {
					return err
				}
				if len(defs) != 1 || defs[0].Type != "web-service" {
					t.Errorf("ListComponentDefinitions result = %+v", defs)
				}
				return nil
			},
		},
		{
			name:       "ListAppComponents",
			bodyReturn: `{"components":[{"id":"c1","name":"api","type":"web-service"}]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/a/a1/c",
			run: func(c *Client) error {
				_, err := c.ListAppComponents(context.Background(), "o1", "a1")
				return err
			},
		},
		{
			name:       "CreateComponent",
			bodyReturn: `{"component":{"id":"c1","name":"api"},"revision":{"id":"r1"}}`,
			wantMethod: http.MethodPost,
			wantPath:   "/organizations/o1/a/a1/c",
			// Values is an empty map, but the request struct tags it
			// omitempty, so it doesn't appear in the wire body. Engine is
			// likewise omitted when empty.
			wantReqBody: `{"name":"api","type":"web-service","environment":"production"}`,
			run: func(c *Client) error {
				resp, err := c.CreateComponent(context.Background(), "o1", "a1", "api", "web-service", "", "production")
				if err != nil {
					return err
				}
				if resp.Component.ID != "c1" {
					t.Errorf("CreateComponent result = %+v", resp)
				}
				return nil
			},
		},
		{
			name:        "CreateComponent_WithEngine",
			bodyReturn:  `{"component":{"id":"c2","name":"web"},"revision":{"id":"r2"}}`,
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/o1/a/a1/c",
			wantReqBody: `{"name":"web","type":"webservice","engine":"helm","environment":"production"}`,
			run: func(c *Client) error {
				_, err := c.CreateComponent(context.Background(), "o1", "a1", "web", "webservice", "helm", "production")
				return err
			},
		},
		{
			name:       "GetComponentInEnv",
			bodyReturn: `{"component_id":"c1","name":"api","environment_name":"production"}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/a/a1/e/production/c/c1",
			run: func(c *Client) error {
				_, err := c.GetComponentInEnv(context.Background(), "o1", "a1", "production", "c1")
				return err
			},
		},
		{
			name:       "ListComponentPods",
			bodyReturn: `{"pods":[{"name":"api-1","phase":"Running","ready":true}]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/a/a1/e/production/c/c1/pods",
			run: func(c *Client) error {
				_, err := c.ListComponentPods(context.Background(), "o1", "a1", "production", "c1")
				return err
			},
		},
		{
			name:       "ListRevisions",
			bodyReturn: `{"revisions":[{"id":"r1","version":1,"status":"deployed"}]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/o1/a/a1/e/production/c/c1/revisions",
			run: func(c *Client) error {
				_, err := c.ListRevisions(context.Background(), "o1", "a1", "production", "c1")
				return err
			},
		},
		{
			name:        "CreateRevision",
			bodyReturn:  `{"id":"r2","version":2,"status":"draft"}`,
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/o1/a/a1/e/production/c/c1/revisions",
			wantReqBody: `{"values":{"replicas":3},"comment":"bump replicas"}`,
			run: func(c *Client) error {
				_, err := c.CreateRevision(context.Background(), "o1", "a1", "production", "c1",
					map[string]interface{}{"replicas": 3}, "bump replicas")
				return err
			},
		},
		{
			name:        "UpdateRevision",
			bodyReturn:  `{"id":"r2","version":2,"status":"draft"}`,
			wantMethod:  http.MethodPut,
			wantPath:    "/organizations/o1/a/a1/e/production/c/c1/revisions/r2",
			wantReqBody: `{"values":{"replicas":4}}`,
			run: func(c *Client) error {
				_, err := c.UpdateRevision(context.Background(), "o1", "a1", "production", "c1", "r2",
					map[string]interface{}{"replicas": 4}, "")
				return err
			},
		},
		{
			name:       "RestartComponent",
			bodyReturn: `{"id":"r9","version":9,"status":"deployed","comment":"Restart at 2026-05-13T12:00:00Z"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/organizations/o1/a/a1/e/production/c/c1/restart",
			run: func(c *Client) error {
				_, err := c.RestartComponent(context.Background(), "o1", "a1", "production", "c1")
				return err
			},
		},
		{
			name:       "DeployLatestDraft",
			bodyReturn: `{"id":"r3","version":3,"status":"deployed"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/organizations/o1/a/a1/e/production/c/c1/deploy",
			run: func(c *Client) error {
				_, err := c.DeployLatestDraft(context.Background(), "o1", "a1", "production", "c1")
				return err
			},
		},
		{
			name:       "DeployRevision",
			bodyReturn: `{"id":"r2","version":2,"status":"deployed"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/organizations/o1/a/a1/e/production/c/c1/revisions/r2/deploy",
			run: func(c *Client) error {
				_, err := c.DeployRevision(context.Background(), "o1", "a1", "production", "c1", "r2")
				return err
			},
		},
		{
			name:        "Promote",
			bodyReturn:  `{"id":"r4","version":4,"status":"draft"}`,
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/o1/a/a1/c/c1/promote",
			wantReqBody: `{"from":"staging","to":"production"}`,
			run: func(c *Client) error {
				_, err := c.Promote(context.Background(), "o1", "a1", "c1", "staging", "production")
				return err
			},
		},
		{
			name:       "ListVariables_org",
			bodyReturn: `[{"id":"v1","name":"FOO","value":"bar","type":"organization","is_encrypted":false}]`,
			wantMethod: http.MethodGet,
			wantPath:   "/variables/o1",
			run: func(c *Client) error {
				vars, err := c.ListVariables(context.Background(), VariableScope{OrgID: "o1"})
				if err != nil {
					return err
				}
				if len(vars) != 1 || vars[0].Name != "FOO" {
					t.Errorf("ListVariables result = %+v", vars)
				}
				return nil
			},
		},
		{
			name:       "ListVariables_env",
			bodyReturn: `[]`,
			wantMethod: http.MethodGet,
			wantPath:   "/variables/o1/a1/e/production",
			run: func(c *Client) error {
				_, err := c.ListVariables(context.Background(), VariableScope{
					OrgID: "o1", ApplicationID: "a1", EnvironmentID: "production",
				})
				return err
			},
		},
		{
			name:       "ListVariables_component",
			bodyReturn: `[]`,
			wantMethod: http.MethodGet,
			wantPath:   "/variables/o1/a1/e/production/c/c1",
			run: func(c *Client) error {
				_, err := c.ListVariables(context.Background(), VariableScope{
					OrgID: "o1", ApplicationID: "a1", EnvironmentID: "production", ComponentID: "c1",
				})
				return err
			},
		},
		{
			name:        "CreateVariable_secret",
			bodyReturn:  `{"id":"v1","name":"DB_PASSWORD","value":"hunter2","is_encrypted":true}`,
			wantMethod:  http.MethodPost,
			wantPath:    "/variables/o1/a1/e/production/c/c1",
			wantReqBody: `{"name":"DB_PASSWORD","value":"hunter2","is_encrypted":true}`,
			run: func(c *Client) error {
				_, err := c.CreateVariable(context.Background(), VariableScope{
					OrgID: "o1", ApplicationID: "a1", EnvironmentID: "production", ComponentID: "c1",
				}, "DB_PASSWORD", "hunter2", true)
				return err
			},
		},
		{
			name:       "DeleteVariable",
			bodyReturn: ``,
			wantMethod: http.MethodDelete,
			wantPath:   "/variables/o1/v1",
			run: func(c *Client) error {
				return c.DeleteVariable(context.Background(), "o1", "v1")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, c := newTestServer(t)
			rec.bodyToReturn = tc.bodyReturn

			if err := tc.run(c); err != nil {
				t.Fatalf("call: %v", err)
			}

			if rec.method != tc.wantMethod {
				t.Errorf("method = %q, want %q", rec.method, tc.wantMethod)
			}
			if rec.path != tc.wantPath {
				t.Errorf("path = %q, want %q", rec.path, tc.wantPath)
			}
			if rec.authCookie != "test-token" {
				t.Errorf("auth cookie = %q, want %q", rec.authCookie, "test-token")
			}
			if tc.wantReqBody != "" {
				assertJSONEqual(t, string(rec.body), tc.wantReqBody)
				if rec.contentType != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", rec.contentType)
				}
			} else {
				// Bodyless requests must not advertise a Content-Type so the
				// server doesn't try to parse an empty buffer.
				if rec.contentType != "" {
					t.Errorf("Content-Type = %q for body-less request, want empty", rec.contentType)
				}
				if len(rec.body) != 0 {
					t.Errorf("body = %q, want empty", string(rec.body))
				}
			}
		})
	}
}

// ---- error path ----------------------------------------------------------

func TestClient_ErrorPathParsesEnvelope(t *testing.T) {
	rec, c := newTestServer(t)
	rec.statusToReturn = http.StatusUnauthorized
	rec.bodyToReturn = `{"code":"1000","error":"unauthorized"}`

	_, err := c.ListOrganizations(context.Background())
	if err == nil {
		t.Fatal("expected error on 401")
	}
	want := "server returned HTTP 401: unauthorized (code 1000)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestClient_ErrorPathFallsBackToRawBody(t *testing.T) {
	rec, c := newTestServer(t)
	rec.statusToReturn = http.StatusBadGateway
	rec.bodyToReturn = `<html>504 from the load balancer</html>`

	_, err := c.ListOrganizations(context.Background())
	if err == nil {
		t.Fatal("expected error on 502")
	}
	if !strings.Contains(err.Error(), "HTTP 502") {
		t.Errorf("error %q should mention HTTP 502", err.Error())
	}
	if !strings.Contains(err.Error(), "load balancer") {
		t.Errorf("error %q should include raw body fallback", err.Error())
	}
}

// ---- context cancellation -------------------------------------------------

func TestClient_RespectsContextCancellation(t *testing.T) {
	// A handler that blocks forever so cancellation is the only way out.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "tok")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := c.ListOrganizations(ctx)
	if err == nil {
		t.Fatal("expected error when context is cancelled mid-flight")
	}
}

// ---- Login (free function, no token yet) ---------------------------------

func TestLogin_HappyPath(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotBody, _ = io.ReadAll(req.Body)
		if req.URL.Path != "/auth/login" || req.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"abc123"}`))
	}))
	t.Cleanup(srv.Close)

	token, err := Login(context.Background(), srv.URL, "user@example.com", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "abc123" {
		t.Errorf("token = %q, want abc123", token)
	}
	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if payload["email"] != "user@example.com" || payload["password"] != "pw" {
		t.Errorf("sent body = %+v", payload)
	}
}

func TestLogin_BadCredentialsRendersEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"1004","error":"invalid_credentials"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := Login(context.Background(), srv.URL, "x", "y")
	if err == nil {
		t.Fatal("expected error")
	}
	want := "login failed (HTTP 401): invalid_credentials (code 1004)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// ---- Stream --------------------------------------------------------------

func TestClient_StreamReturnsBodyOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/logs" {
			t.Errorf("path = %q", req.URL.Path)
		}
		if req.URL.Query().Get("follow") != "true" {
			t.Errorf("query = %q, want follow=true", req.URL.RawQuery)
		}
		_, _ = w.Write([]byte("line 1\nline 2\n"))
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "tok")

	body, err := c.Stream(context.Background(), "/logs", url.Values{"follow": []string{"true"}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	t.Cleanup(func() { _ = body.Close() })
	data, _ := io.ReadAll(body)
	if string(data) != "line 1\nline 2\n" {
		t.Errorf("body = %q", string(data))
	}
}

func TestClient_StreamReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"4005","error":"pod_not_found"}`))
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "tok")

	_, err := c.Stream(context.Background(), "/logs", nil)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	want := "server returned HTTP 404: pod_not_found (code 4005)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// ---- helpers -------------------------------------------------------------

// assertJSONEqual fails the test when two JSON strings disagree after
// normalisation. Comparing raw bytes is brittle because field order in a
// map is non-deterministic in Go's JSON encoder when the source is a
// map[string]any.
func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("got is not valid JSON: %v (raw=%q)", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want is not valid JSON: %v (raw=%q)", err, want)
	}
	gn, _ := json.Marshal(g)
	wn, _ := json.Marshal(w)
	if string(gn) != string(wn) {
		t.Errorf("body mismatch:\n got  = %s\n want = %s", got, want)
	}
}
