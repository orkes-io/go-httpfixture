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
			reqMethod: http.MethodGet,
			reqPath:   "/path",
			reqBody:   nil,
			fixture:   httpfixture.GetOK("/path", "hello world"),
			wantBody:  "hello world",
			wantCode:  http.StatusOK,
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
