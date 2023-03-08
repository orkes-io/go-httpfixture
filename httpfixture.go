package httpfixture

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// F is a simple HTTP fixture.
type F interface {
	// Run runs this fixture, exchanging the provided request for a response.
	Run(t *testing.T, req *http.Request) *http.Response
	// Route returns the route where this Fixture is hosted.
	Route() string
	// Method returns the method which this Fixture matches on.
	Method() string
}

type FixtureOpt func(f *baseFixture)

// GetOK returns a fixture which responds to GET requests at the provided route with the provided response body, and
// // status 200 OK.
func GetOK(route string, body string, opts ...FixtureOpt) F {
	return GetBytesOK(route, []byte(body), opts...)
}

// GetBytesOK returns a fixture which responds to GET requests at the provided route with the provided body,  and status
// 200 OK.
func GetBytesOK(route string, body []byte, opts ...FixtureOpt) F {
	return BytesOK(route, http.MethodGet, body, opts...)
}

// BytesOK returns a fixture which responds to requests at the provided route and HTTP method with the provided body,
// and status 200 OK.
func BytesOK(route string, method string, body []byte, opts ...FixtureOpt) F {
	return Bytes(route, method, http.StatusOK, body, opts...)
}

// Bytes returns a fixture which responds to requests with the provided route and HTTP method with the provided body and
// status code.
func Bytes(route, method string, responseCode int, body []byte, opts ...FixtureOpt) F {
	result := &memFixture{
		body: body,
		baseFixture: baseFixture{
			method:       method,
			route:        standardizePath(route),
			responseCode: responseCode,
		},
	}
	for _, opt := range opts {
		opt(&result.baseFixture)
	}
	return result
}

// SmallFixture is for fixtures whose bodies fit in memory.
type memFixture struct {
	body []byte
	baseFixture
}

// Run exchanges the provided request for an appropriate response.
func (s *memFixture) Run(t *testing.T, req *http.Request) *http.Response {
	if err := s.baseFixture.AssertAll(req); err != nil {
		t.Fatalf("request failed assertions: %v", err)
		return nil
	}
	resp := s.baseFixture.Response()
	resp.Body = io.NopCloser(bytes.NewBuffer(s.body))
	return resp
}

func (s *memFixture) Route() string {
	return s.route
}

func (s *memFixture) Method() string {
	return s.method
}

type baseFixture struct {
	route        string
	method       string
	responseCode int
	assertions   []assert
}

func (bf *baseFixture) AssertAll(req *http.Request) error {
	var errs error
	for _, a := range bf.assertions {
		if err := a(req); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (bf *baseFixture) Response() *http.Response {
	return &http.Response{
		StatusCode: bf.responseCode,
	}
}

type Server struct {
	*httptest.Server
	t      *testing.T
	routes map[string]F
}

// NewServer creates a new httpfixture.Server which responds to requests with the provided fixtures.
func NewServer(fixtures ...F) *Server {
	result := &Server{
		routes: make(map[string]F),
	}
	result.Server = httptest.NewUnstartedServer(result)
	for _, f := range fixtures {
		result.routes[routesKey(f.Method(), f.Route())] = f
	}
	return result
}

// Start starts the server, reporting assertions using the provided testing.T.
func (s *Server) Start(t *testing.T) {
	s.t = t
	s.Server.Start()
}

// StartTLS starts the server in TLS mode, reporting assertions using the provided testing.T.
func (s *Server) StartTLS(t *testing.T) {
	s.t = t
	s.Server.StartTLS()
}

func (s *Server) Close() {
	s.Server.Close()
}

func (s *Server) URL() string {
	return s.Server.URL
}

type assert func(req *http.Request) error

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL == nil {
		s.t.Fatalf("nil request URL")
		return
	}
	f, ok := s.routes[routesKey(req.Method, req.URL.Path)]
	if !ok {
		http.NotFound(rw, req)
		return
	}
	resp := f.Run(s.t, req)
	if resp == nil {
		return
	}
	for key, vals := range resp.Header {
		for _, v := range vals {
			resp.Header.Add(key, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(rw, resp.Body); err != nil {
		s.t.Fatalf("failed to copy response body: %v", err)
		return
	}
	return
}

func routesKey(method, route string) string {
	return fmt.Sprintf("%s:%s", method, route)
}

func standardizePath(path string) string {
	if len(path) == 0 {
		return "/"
	}
	if path[0] == '/' {
		return path
	}
	return fmt.Sprintf("/%s", path)
}
