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

func GetOK(route string, body string, opts ...FixtureOpt) F {
	return GetBytesOK(route, []byte(body), opts...)
}

func GetBytesOK(route string, body []byte, opts ...FixtureOpt) F {
	return BytesOK(route, http.MethodGet, body, opts...)
}

func BytesOK(route string, method string, body []byte, opts ...FixtureOpt) F {
	return Bytes(route, method, http.StatusOK, body, opts...)
}

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
	resp.Body = &buffCloser{Buffer: bytes.NewBuffer(s.body)}
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
	s      *httptest.Server
	t      *testing.T
	routes map[string]F
}

func NewServer(fixtures ...F) *Server {
	result := &Server{
		routes: make(map[string]F),
	}
	result.s = httptest.NewUnstartedServer(result)
	for _, f := range fixtures {
		result.routes[routesKey(f.Method(), f.Route())] = f
	}
	return result
}

func (s *Server) Start(t *testing.T) {
	s.t = t
	s.s.Start()
}

func (s *Server) StartTLS(t *testing.T) {
	s.t = t
	s.s.StartTLS()
}

func (s *Server) Close() {
	s.s.Close()
}

func (s *Server) URL() string {
	return s.s.URL
}

type assert func(req *http.Request) error

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

// buffCloser creates a small io.ReadCloser around a bytes.Buffer.
type buffCloser struct {
	*bytes.Buffer
}

func (bc *buffCloser) Read(p []byte) (int, error) {
	return bc.Buffer.Read(p)
}

func (bc *buffCloser) Close() error {
	return nil
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
