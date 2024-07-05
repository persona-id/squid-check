package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// expectHttp is a struct to hold the expected http responses
type expectHttp struct {
	body       string
	header     http.Header
	statusCode int
}

// mockProxy is a handler to mock the squid proxy
func mockProxy(w http.ResponseWriter, r *http.Request) {
	// Create a new HTTP request with the same method, URL, and body as the original request
	targetURL := r.URL
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	// Copy the headers from the original request to the proxy request
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Send the proxy request
	transport := http.DefaultTransport
	resp, err := transport.RoundTrip(proxyReq)
	if err != nil {
		http.Error(w, "Error sending proxy request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy the headers from the proxy response to the original response
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// add a header to the response to indicate that the response is from the proxy
	w.Header().Add("X-Proxy", "true")

	// Set the status code of the original response to the status code of the proxy response
	w.WriteHeader(resp.StatusCode)

	// Copy the body of the proxy response to the original response
	io.Copy(w, resp.Body)
}

// TestNewBuildInfo suite to test creating a new buildInfo struct
func TestNewBuildInfo(t *testing.T) {
	cases := []struct {
		name     string
		expected *buildInfo
	}{
		{
			name: "returnsBuildInfoWithDefaultValues",
			expected: &buildInfo{
				goVersion: "unknown",
				version:   "unknown",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newBuildInfo()

			// compare expected to actual
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("got %v; expect %v", got, tc.expected)
			}
		})
	}
}

// TestNewProxyClient suite to test creating a new proxy client
func TestNewProxyClient(t *testing.T) {
	cases := []struct {
		name, expected string
	}{
		{
			name:     "returnsClientWithLocalhostProxyAddress",
			expected: "http://127.0.0.1:3128",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// parse expected address to get host:port component for calling newProxyClient
			expectedParsed, _ := url.Parse(tc.expected)

			// create fake request to see if we have a proxy configured
			fakeReq := httptest.NewRequest(http.MethodGet, "/", nil)

			// call newProxyClient
			client, err := newProxyClient(expectedParsed.Host)
			if err != nil {
				t.Errorf("got %v; expect nil", err)
			}

			gotProxy := client.Transport.(*http.Transport).Proxy
			gotProxyURL, _ := gotProxy(fakeReq)

			// compare proxy address to what is expected
			if gotProxyURL.Scheme != expectedParsed.Scheme {
				t.Errorf("got %v; expect %v", gotProxyURL.Scheme, expectedParsed.Scheme)
			}

			if gotProxyURL.Host != expectedParsed.Host {
				t.Errorf("got %v; expect %v", gotProxyURL.Host, expectedParsed.Host)
			}
		})
	}
}

// TestHealthzHandler suite to test the /healthz handler
func TestHealthzHandler(t *testing.T) {
	cases := []struct {
		name             string
		expect           expectHttp
		customTargetPath string

	}{
		{
			name: "returnsSuccessWithHttp200",
			expect: expectHttp{
				body: "success",
				header: map[string][]string{
					"Cache-Control": {"no-store"},
					"Content-Type":  {"text/plain; charset=utf-8"},
				},
				statusCode: http.StatusOK,
			},
			customTargetPath: "/target",  // default
		},
		{
			name: "returnsSuccessWithHttp200",
			expect: expectHttp{
				body: "success",
				header: map[string][]string{
					"Cache-Control": {"no-store"},
					"Content-Type":  {"text/plain; charset=utf-8"},
				},
				statusCode: http.StatusOK,
			},
			customTargetPath: "/differentEndpoint",  // default
		},

	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// mock the /target endpoint
			mockTarget := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("success"))

						// check that only /target is called and with verb GET
						if r.Method != http.MethodGet {
							t.Errorf("got %v; expect %v", r.Method, http.MethodGet)
						}

						if r.URL.Path != tc.customTargetPath {
							t.Errorf("got %v; expect %v", r.URL.Path, tc.customTargetPath)
						}
					},
				),
			)
			defer mockTarget.Close()

			// mock the squid proxy
			mockProxy := httptest.NewServer(http.HandlerFunc(mockProxy))

			flags.targetAddress = mockTarget.Listener.Addr().String()
			flags.targetPath = tc.customTargetPath
			flags.proxyAddress = mockProxy.Listener.Addr().String()
			proxyAddress, _ := url.Parse(fmt.Sprintf("http://%s", flags.proxyAddress))

			// setup http request for /healthz
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

			client := &checkConfig{
				proxyClient: &http.Client{
					Transport: &http.Transport{
						Proxy: http.ProxyURL(proxyAddress),
					},
				},
			}

			// call healthzHandler
			client.healthzHandler()(rr, req)
			res := rr.Result()
			resBody, _ := io.ReadAll(res.Body)

			// check if we connected to the mock target via the mock proxy by checking for the X-Proxy header
			if res.Header.Get("X-Proxy") != "true" {
				t.Error("request was not proxied")
			}

			// compare response body
			if string(resBody) != tc.expect.body {
				t.Errorf("got %v; expect %v", string(resBody), tc.expect.body)
			}

			// compare response status code
			if res.StatusCode != tc.expect.statusCode {
				t.Errorf("got %v; expect %v", res.StatusCode, tc.expect.statusCode)
			}
		})
	}
}

// TestTargetHandler suite to test the /target handler
func TestTargetHandler(t *testing.T) {
	cases := []struct {
		name   string
		expect expectHttp
	}{
		{
			name: "returnsSuccessWithHttp200",
			expect: expectHttp{
				body: "success",
				header: map[string][]string{
					"Cache-Control": {"no-store"},
					"Content-Type":  {"text/plain; charset=utf-8"},
				},
				statusCode: http.StatusOK,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// setup http request for /target
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/target", nil)

			// call targetHandler
			targetHandler()(rr, req)
			res := rr.Result()
			resBody, _ := io.ReadAll(res.Body)

			// compare response body
			if string(resBody) != tc.expect.body {
				t.Errorf("got %v; expect %v", string(resBody), tc.expect.body)
			}

			// compare response headers
			if !reflect.DeepEqual(res.Header, tc.expect.header) {
				t.Errorf("got %v; expect %v", res.Header, tc.expect.header)
			}

			// compare response status code
			if res.StatusCode != tc.expect.statusCode {
				t.Errorf("got %v; expect %v", res.StatusCode, tc.expect.statusCode)
			}
		})
	}
}
