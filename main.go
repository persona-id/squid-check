package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

// cliFlags is a struct to hold the cli flags
type cliFlags struct {
	listenAddress, proxyAddress, targetAddress string
}

// checkConfig is a struct to hold the config for the http handlers
type checkConfig struct {
	proxyClient *http.Client
}

// flags holds the global set cli flags
var flags cliFlags = cliFlags{}

func init() {
	flag.StringVar(&flags.listenAddress, "listen-address", "0.0.0.0:8080", "Address to listen on")
	flag.StringVar(&flags.proxyAddress, "proxy-address", "127.0.0.1:3128", "Address of squid proxy")
	flag.StringVar(&flags.targetAddress, "target-address", "127.0.0.1:8080", "Address of proxied health check target")
}

// newProxyClient returns a new http client configured to use squid
func newProxyClient(address string) (*http.Client, error) {
	// parse address of squid server to monitor health
	proxyUrl, err := url.Parse(fmt.Sprintf("http://%s", address))
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}, nil
}

// healthzHandler is a handler for the /healthz uri
// It makes a proxied connection through squid back to the /target uri served
// by this application. If the connection is successful, it returns a 200 OK
func (s *checkConfig) healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// request /target via the proxy client
		resp, err := s.proxyClient.Get(fmt.Sprintf("http://%s/target", flags.targetAddress))
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("error connecting to /target"))
			log.Println(err)
		}
		defer resp.Body.Close()

		// write the response body to the client
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)

		// copy headers from response to requestor
		for k, v := range resp.Header {
			w.Header()[k] = v
		}

		// copy http status code
		w.WriteHeader(resp.StatusCode)

		// copy body
		io.Copy(w, resp.Body)
	}
}

// targetHandler is a handler for the /target uri
// It returns a 200 OK response with the body "healthy"
func targetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)

		// prevent proxies from caching the response
		// this will help ensure squid will always make a request to the target
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte("success"))
	}
}

func main() {
	// setup cli flags
	flag.Parse()

	// create new squid client
	proxyUrl, err := newProxyClient(flags.proxyAddress)
	if err != nil {
		log.Fatalf("error creating proxy client: %v", err)
	}

	// build checkConfig for handlers
	checkConfig := &checkConfig{
		proxyClient: proxyUrl,
	}

	// setup http mux
	mux := http.NewServeMux()

	// setup http endpoint handlers
	mux.Handle("/healthz", checkConfig.healthzHandler())
	mux.Handle("/target", targetHandler())

	// start http server
	log.Print("Listening...")
	// this is a health check service, so we don't want to use TLS
	// nosemgrep: go.lang.security.audit.net.use-tls.use-tls
	log.Fatal(http.ListenAndServe(fmt.Sprint(flags.listenAddress), mux))
}
