package httpfixture_test

import (
	"bytes"
	"fmt"
	"github.com/orkes-io/go-httpfixture"
	"io"
	"net/http"
	"testing"
)

func TestFixture(t *testing.T) {
	tests := []struct {
		name      string
		reqMethod string
		reqPath   string
		reqBody   []byte
		fixture   httpfixture.F
		wantBody  string
		wantCode  int
	}{
		{
			name:      "OK wildcard POST",
			reqMethod: http.MethodPost,
			reqPath:   "/path/subpath",
			reqBody:   nil,
			fixture:   httpfixture.OK("/path", "hello world"),
			wantBody:  "hello world",
			wantCode:  http.StatusOK,
		},
		{
			name:      "OK wildcard DELETE",
			reqMethod: http.MethodDelete,
			reqPath:   "/path",
			reqBody:   nil,
			fixture:   httpfixture.OK("/path", "hello world"),
			wantBody:  "hello world",
			wantCode:  http.StatusOK,
		},
		{
			name:      "GetOK",
			reqMethod: http.MethodGet,
			reqPath:   "/path",
			reqBody:   nil,
			fixture:   httpfixture.GetOK("/path", "hello world"),
			wantBody:  "hello world",
			wantCode:  http.StatusOK,
		},
		{
			name:      "GetOK with subpath",
			reqMethod: http.MethodGet,
			reqPath:   "/path/subpath",
			reqBody:   nil,
			fixture:   httpfixture.GetOK("/path", "hello world"),
			wantBody:  "hello world",
			wantCode:  http.StatusOK,
		},
		{
			name:      "GetOK",
			reqMethod: http.MethodGet,
			reqPath:   "/path",
			reqBody:   nil,
			fixture:   httpfixture.GetOK("/path", "hello world"),
			wantBody:  "hello world",
			wantCode:  http.StatusOK,
		},
		{
			name:      "GetBytesOK",
			reqMethod: http.MethodGet,
			reqPath:   "/other/path",
			reqBody:   nil,
			fixture:   httpfixture.GetBytesOK("/other/path", []byte("some bytes")),
			wantBody:  "some bytes",
			wantCode:  http.StatusOK,
		},
		{
			name:      "BytesOK",
			reqMethod: http.MethodDelete,
			reqPath:   "/other/path",
			reqBody:   nil,
			fixture:   httpfixture.BytesOK("/other/path", http.MethodDelete, []byte("moar bytes")),
			wantBody:  "moar bytes",
			wantCode:  http.StatusOK,
		},
		{
			name:      "Bytes",
			reqMethod: http.MethodDelete,
			reqPath:   "/other/path",
			reqBody:   nil,
			fixture:   httpfixture.Bytes("/other/path", http.MethodDelete, http.StatusCreated, []byte("moar bytes")),
			wantBody:  "moar bytes",
			wantCode:  http.StatusCreated,
		},
		{
			name:      "GetFileOK",
			reqMethod: http.MethodGet,
			reqPath:   "/path2",
			reqBody:   nil,
			fixture:   httpfixture.GetFileOK("/path2", "testdata/basic-body.json"),
			wantBody:  `{"foo":"bar","number":1}`,
			wantCode:  http.StatusOK,
		},
		{
			name:      "FileOK",
			reqMethod: http.MethodPost,
			reqPath:   "/path2",
			reqBody:   nil,
			fixture:   httpfixture.FileOK("/path2", http.MethodPost, "testdata/basic-body.json"),
			wantBody:  `{"foo":"bar","number":1}`,
			wantCode:  http.StatusOK,
		},
		{
			name:      "FileOK",
			reqMethod: http.MethodPost,
			reqPath:   "/path2",
			reqBody:   nil,
			fixture:   httpfixture.File("/path2", http.MethodPost, http.StatusAccepted, "testdata/basic-body.json"),
			wantBody:  `{"foo":"bar","number":1}`,
			wantCode:  http.StatusAccepted,
		},
		{
			name:      "NotFound",
			reqMethod: http.MethodDelete,
			reqPath:   "/path2",
			reqBody:   nil,
			fixture:   httpfixture.NotFound("/path2", http.MethodDelete),
			wantCode:  http.StatusNotFound,
		},
		{
			name:      "ResponseCode",
			reqMethod: http.MethodPut,
			reqPath:   "/path",
			reqBody:   nil,
			fixture:   httpfixture.ResponseCode("/path", http.MethodPut, 755),
			wantCode:  755,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httpfixture.NewServer(tt.fixture)
			s.Start(t)
			defer s.Close()

			req, err := http.NewRequest(tt.reqMethod, fmt.Sprintf("%s%s", s.URL(), tt.reqPath), bytes.NewBuffer(tt.reqBody))
			if err != nil {
				t.Fatalf("error creating request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("error making request: %v", err)
			}

			if resp.StatusCode != tt.wantCode {
				t.Fatalf("want statusCode: %d; got: %d", tt.wantCode, resp.StatusCode)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("error reading body from response: %v", err)
			}
			bytes.Equal([]byte(tt.wantBody), body)
		})
	}
}

func TestSeq(t *testing.T) {
	s := httpfixture.NewServer(httpfixture.Seq("/path", "GET",
		httpfixture.OK("", "body1"),
		httpfixture.OK("", "body2"),
		httpfixture.OK("", "body3"),
		httpfixture.OK("", "body4"),
	))
	s.Start(t)
	defer s.Close()

	for i := 0; i < 4; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/path", s.URL()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		actualBody := string(must(io.ReadAll(resp.Body)))
		if actualBody != fmt.Sprintf("body%d", i+1) {
			t.Fatalf("want: body%d; got: '%v'", i+1, actualBody)
		}
	}
	for i := 0; i < 5; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/path", s.URL()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		actualBody := string(must(io.ReadAll(resp.Body)))
		if actualBody != "body4" {
			t.Fatalf("want: 'body4'; got: '%v'", actualBody)
		}
	}
}

func TestFixtureAssertions(t *testing.T) {
	tests := []struct {
		name        string
		req         *http.Request
		fixture     httpfixture.F
		wantFailure bool
	}{
		{
			name: "AssertBodyContains",
			req: must(http.NewRequest("GET", "http://localhost:8080/path",
				bytes.NewBufferString("the quick brown fox jumped over the"))),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertBodyContains("quick brown fox")),
		},
		{
			name: "AssertBodyContains failure",
			req: must(http.NewRequest("GET", "http://localhost:8080/path/another",
				bytes.NewBufferString("something else even"))),
			fixture: httpfixture.GetOK("/path/another", "response body",
				httpfixture.AssertBodyContains("quick brown fox")),
			wantFailure: true,
		},
		{
			name: "AssertBodyContainsBytes",
			req: must(http.NewRequest("GET", "http://localhost:8080/path",
				bytes.NewBufferString("some text\n\n\rother text here"))),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertBodyContainsBytes([]byte("\n\n\r"))),
		},
		{
			name: "AssertBodyContainsBytes failure",
			req: must(http.NewRequest("GET", "http://localhost:8080/path",
				bytes.NewBufferString(""))),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertBodyContainsBytes([]byte("o"))),
			wantFailure: true,
		},
		{
			name: "AssertHeaderMatches",
			req: withHeader(must(http.NewRequest("GET", "http://localhost:7070/path", nil)),
				"Content-Type", "application/json"),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertHeaderMatches("Content-Type", "application/json")),
		},
		{
			name: "AssertHeaderMatches failure",
			req:  must(http.NewRequest("GET", "http://localhost:7070/path", nil)),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertHeaderMatches("Content-Type", "application/json")),
			wantFailure: true,
		},
		{
			name: "AssertURLContains",
			req:  must(http.NewRequest("GET", "http://localhost:7070/tasks/1234/status", nil)),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertURLContains("tasks/1234/")),
		},
		{
			name: "AssertURLContains",
			req:  must(http.NewRequest("GET", "http://localhost:7070/tasks/1234/status", nil)),
			fixture: httpfixture.GetOK("/path", "",
				httpfixture.AssertURLContains("tasks/4567/")),
			wantFailure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				err := recover()
				if err != nil && !tt.wantFailure {
					t.Fatalf("caught panic in test: %v", err)
				}
			}()
			testT := &testing.T{}
			_ = tt.fixture.Run(testT, tt.req)

			if tt.wantFailure != testT.Failed() {
				t.Fatalf("unexpected failure reported; want: %t; got: %t", tt.wantFailure, testT.Failed())
			}
		})
	}
}

func withHeader(req *http.Request, key, value string) *http.Request {
	req.Header.Add(key, value)
	return req
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(fmt.Errorf("must had error: %v", err))
	}
	return t
}
