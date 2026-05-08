package component

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// logRegistryPullFailure replays the registry auth + manifest fetch that
// go-containerregistry / Timoni just performed, but with redirects disabled
// so we can log exactly what each hop returned (status, Location,
// WWW-Authenticate, body). This is intended to be called only on pull
// failure — it's a diagnostic, not a hot path.
//
// creds is the same "user:password" string passed to Timoni; ociRepository is
// e.g. "ghcr.io/org/repo". The probe does up to 6 redirect hops.
func logRegistryPullFailure(ctx context.Context, logger logr.Logger, ociRepository, ociTag, creds string) {
	repo := strings.TrimPrefix(ociRepository, "oci://")
	host := registryHost(repo)
	path := strings.TrimPrefix(repo, host)
	path = strings.TrimPrefix(path, "/")

	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, path, ociTag)
	logger.Info("registry probe: starting", "manifestURL", manifestURL, "credsProvided", creds != "")

	// Step 1: hit the manifest endpoint anonymously. Expect 401 with a
	// WWW-Authenticate header pointing at the token service.
	resp1, body1, err := probeRequest(ctx, manifestURL, "")
	if err != nil {
		logger.Error(err, "registry probe: manifest request failed", "url", manifestURL)
		return
	}
	logger.Info("registry probe: manifest (anonymous)",
		"url", manifestURL,
		"status", resp1.StatusCode,
		"location", resp1.Header.Get("Location"),
		"wwwAuthenticate", resp1.Header.Get("WWW-Authenticate"),
		"body", truncate(body1, 256))

	// Parse Bearer challenge, e.g.:
	//   Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:org/repo:pull"
	tokenURL, ok := buildTokenURL(resp1.Header.Get("WWW-Authenticate"))
	if !ok {
		logger.Info("registry probe: no Bearer challenge — stopping")
		return
	}

	// Step 2a: hit the token endpoint WITHOUT credentials. If go-containerregistry
	// is somehow stripping the Authorization header, this is the request shape
	// the OCI client is actually issuing — and any redirect loop here is the
	// real source of the "stopped after 10 redirects" error.
	logger.Info("registry probe: probing token URL anonymously to compare against OCI client behavior")
	currentAnon := tokenURL
	for hop := 1; hop <= 6; hop++ {
		resp, body, err := probeRequest(ctx, currentAnon, "")
		if err != nil {
			logger.Error(err, "registry probe: anonymous token hop failed", "hop", hop, "url", currentAnon)
			break
		}
		logger.Info("registry probe: token hop (anonymous)",
			"hop", hop,
			"url", currentAnon,
			"status", resp.StatusCode,
			"location", resp.Header.Get("Location"),
			"wwwAuthenticate", resp.Header.Get("WWW-Authenticate"),
			"body", truncate(body, 256))
		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			break
		}
		loc := resp.Header.Get("Location")
		if loc == "" {
			break
		}
		next, err := resolveRedirect(currentAnon, loc)
		if err != nil {
			logger.Error(err, "registry probe: failed to resolve anonymous redirect", "hop", hop, "location", loc)
			break
		}
		currentAnon = next
	}

	// Step 2b: hit the token endpoint with credentials, following each redirect
	// manually so we can log it.
	current := tokenURL
	for hop := 1; hop <= 6; hop++ {
		resp, body, err := probeRequest(ctx, current, creds)
		if err != nil {
			logger.Error(err, "registry probe: token hop failed", "hop", hop, "url", current)
			return
		}
		logger.Info("registry probe: token hop",
			"hop", hop,
			"url", current,
			"status", resp.StatusCode,
			"location", resp.Header.Get("Location"),
			"wwwAuthenticate", resp.Header.Get("WWW-Authenticate"),
			"body", truncate(body, 512))

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				logger.Info("registry probe: redirect with empty Location — stopping", "hop", hop)
				return
			}
			next, err := resolveRedirect(current, loc)
			if err != nil {
				logger.Error(err, "registry probe: failed to resolve redirect", "hop", hop, "location", loc)
				return
			}
			current = next
			continue
		}
		// Non-redirect response (2xx, 4xx, 5xx). Done.
		return
	}
	logger.Info("registry probe: exhausted hop budget (6) — same redirect loop the OCI client sees")
}

func probeRequest(ctx context.Context, target, creds string) (*http.Response, []byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, err
	}
	if creds != "" {
		// Match what go-containerregistry does: send Basic on the token
		// endpoint to exchange for a Bearer.
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	}
	req.Header.Set("User-Agent", "conure-controller/registry-probe")

	client := &http.Client{
		// Disable Go's automatic redirect following so we can log each hop.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 15 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp, body, nil
}

func buildTokenURL(wwwAuthenticate string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(wwwAuthenticate), "bearer ") {
		return "", false
	}
	params := parseBearerParams(wwwAuthenticate[len("Bearer "):])
	realm := params["realm"]
	if realm == "" {
		return "", false
	}
	q := url.Values{}
	if s := params["service"]; s != "" {
		q.Set("service", s)
	}
	if s := params["scope"]; s != "" {
		q.Set("scope", s)
	}
	if len(q) == 0 {
		return realm, true
	}
	return realm + "?" + q.Encode(), true
}

func parseBearerParams(s string) map[string]string {
	out := map[string]string{}
	for _, part := range splitOutsideQuotes(s, ',') {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(part[:eq])
		v := strings.TrimSpace(part[eq+1:])
		v = strings.Trim(v, `"`)
		out[strings.ToLower(k)] = v
	}
	return out
}

func splitOutsideQuotes(s string, sep byte) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
			cur.WriteByte(c)
			continue
		}
		if c == sep && !inQuote {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func resolveRedirect(base, location string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	l, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(l).String(), nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}
