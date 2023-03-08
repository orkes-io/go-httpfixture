package httpfixture

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// F is an HTTP fixture.
type F interface {
	// Run runs this fixture, exchanging the provided request for a response.
	Run(t *testing.T, req *http.Request) *http.Response
	// Route returns the route where this Fixture is hosted.
	Route() string
	// Method returns the method which this Fixture matches on.
	Method() string
}

// FixtureOpt represents an optional parameter added to a fixture, usually request assertions.
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
	return &memFixture{
		body:        body,
		baseFixture: base(route, method, responseCode, opts...),
	}
}

// GetFileOK returns a fixture which responds to GET requests at the provided route with the contents of the provided
// file and status 200 OK. The file at the provided path is read into memory by this func.
func GetFileOK(route, path string, opts ...FixtureOpt) F {
	return FileOK(route, http.MethodGet, path, opts...)
}

// FileOK returns a fixture which responds to matching requests with the contents of the provided file and status 200
// OK. The provided file is read into memory by this func.
func FileOK(route, method string, path string, opts ...FixtureOpt) F {
	return File(route, method, http.StatusOK, path, opts...)
}

// File returns a fixture which responds to matching requests with the contents of the provided file, which are read
// into memory by this func.
func File(route, method string, responseCode int, path string, opts ...FixtureOpt) F {
	f, err := os.Open(path)
	if err != nil {
		panic(fmt.Errorf("error reading file: %w", err))
	}
	defer f.Close()
	return Reader(route, method, responseCode, f, opts...)
}

// Reader returns a fixture which responds to matching requests with the contents of the provided reader, which are read
// into memory by this func.
func Reader(route, method string, responseCode int, reader io.Reader, opts ...FixtureOpt) F {
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("error reading reader: %w", err))
	}
	return &memFixture{
		body:        b,
		baseFixture: base(route, method, responseCode, opts...),
	}
}

func base(route, method string, responseCode int, opts ...FixtureOpt) baseFixture {
	bf := baseFixture{
		method:       method,
		route:        standardizePath(route),
		responseCode: responseCode,
	}
	for _, opt := range opts {
		opt(&bf)
	}
	return bf
}

// AssertHeaderMatches asserts that the provided key, value pair is present in the headers of any incoming request.
func AssertHeaderMatches(key, value string) FixtureOpt {
	return func(f *baseFixture) {
		f.assertions = append(f.assertions, func(req *http.Request) error {
			vals := req.Header.Values(key)
			for _, v := range vals {
				if strings.EqualFold(v, value) {
					return nil
				}
			}
			return fmt.Errorf("could not find headers matching %s: %s", key, value)
		})
	}
}

// AssertBodyContains asserts all requests passed to this fixture include a body containing the provided string.
func AssertBodyContains(str string) FixtureOpt {
	return AssertBodyContainsBytes([]byte(str))
}

// AssertBodyContainsBytes asserts all requests passed to this fixture contains the provided byte sequence in their
// body.
func AssertBodyContainsBytes(b []byte) FixtureOpt {
	return func(f *baseFixture) {
		f.assertions = append(f.assertions, func(req *http.Request) error {
			body := bytes.NewBuffer(make([]byte, req.ContentLength))
			r := io.TeeReader(req.Body, body)
			req.Body = io.NopCloser(body)
			bodyBytes, err := io.ReadAll(r)
			if err != nil {
				return fmt.Errorf("error reading request body: %w", err)
			}
			if !bytes.Contains(bodyBytes, b) {
				return errors.New("body did not contain expected bytes")
			}
			return nil
		})
	}
}

// memFixture is for fixtures whose response bodies fit in memory.
type memFixture struct {
	body []byte
	baseFixture
}

// Run exchanges the provided request for an appropriate response.
func (s *memFixture) Run(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	s.baseFixture.assertAll(t, req)
	resp := s.baseFixture.response()
	resp.Body = io.NopCloser(bytes.NewBuffer(s.body))
	return resp
}

type baseFixture struct {
	route        string
	method       string
	responseCode int
	assertions   []assert
}

// assertAll runs all request assertions against the provided incoming request. It fails and halts the current test if
// any assertion fails.
func (bf *baseFixture) assertAll(t *testing.T, req *http.Request) {
	t.Helper()
	var errs error
	for _, a := range bf.assertions {
		if err := a(req); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		t.Logf("request failed assertions: %v", errs)
		t.Fail()
		return
	}
}

// Response creates a new response populated with fields set in this baseFixture.
func (bf *baseFixture) response() *http.Response {
	return &http.Response{
		StatusCode: bf.responseCode,
	}
}

// Route returns the route used to trigger this fixture.
func (bf *baseFixture) Route() string {
	return bf.route
}

// Method returns the HTTP method used to trigger this fixture.
func (bf *baseFixture) Method() string {
	return bf.method
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

// Close closes the underlying httptest.Server.
func (s *Server) Close() {
	s.Server.Close()
}

// URL retrieves the URL of this server, once it's been started.
func (s *Server) URL() string {
	return s.Server.URL
}

type assert func(req *http.Request) error

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL == nil {
		s.t.Logf("nil request URL")
		s.t.Fail()
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
		s.t.Logf("failed to copy response body: %v", err)
		s.t.Fail()
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
