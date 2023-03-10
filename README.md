# orkes-io/go-httpfixture
Go helpers for unit testing using dead simple HTTP fixtures.

## Usage

#### Installation via `go get`
```shell
go get github.com/orkes-io/go-httpfixture
```

This package provides logicless HTTP fixtures which provide a fixed response to requests, optionally asserting that the
request matches an expected form.

#### Basic usage in a Go unit test:

```go
package example

import (
	"testing"
	"net/http"
	"github.com/orkes-io/go-httpfixture"
)

func TestHTTP(t *testing.T) {
	s := httpfixture.NewServer(
		httpfixture.GetOK("/api/example", `{"response":"hello fixture"}`),
    )
	s.Start(t)
	defer s.Close()
	
	resp, err := http.Get(s.URL() + "/api/example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
	    t.Fatalf("expected 200 OK; got: %s", resp.StatusCode)	
    }
}
```

The above test starts a server which contains a single fixture, makes a GET request against the fixture, and asserts the
success completed succesfully.

httpfixture serves as a terser alternative to [net/http/httptest](pkg.go.dev/net/http/httptest), which may be better
suited to table-driven tests. All contributions are welcome! Please read the 
[Contributor Covenant Code of Conduct](https://github.com/orkes-io/.github/blob/main/CODE_OF_CONDUCT.md) prior to
contributing.

