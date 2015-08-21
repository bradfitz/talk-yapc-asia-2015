# Profiling & Optimizing in Go

Brad Fitzpatrick

YAPC::Asia 2015

Tokyo Big Sight, 2015-08-22

## Starting program.

Let's debug and optimize a simple HTTP server.

```
package main

import (
        "fmt"
        "log"
        "net/http"
        "regexp"
)

var visitors int

func handleHi(w http.ResponseWriter, r *http.Request) {
        if match, _ := regexp.MatchString(`^\w*$`, r.FormValue("color")); !match {
                http.Error(w, "Optional color is invalid", http.StatusBadRequest)
                return
        }
        visitors++
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte("<h1 style='color: " + r.FormValue("color") + "'>Welcome!</h1>You are visitor number " + fmt.Sprint(visitors) + "!"))
}

func main() {
        log.Printf("Starting on port 8080")
        http.HandleFunc("/hi", handleHi)
        log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
```

### Run it.

```
$ cd $GOPATH/src/github.com/bradfitz/talk-yapc-asia-2015/demo
$ go run yapc.go
or
$ go build && ./demo
or
$ go install && demo
```

### Testing

```
$ go test
?       yapc/demo       [no test files]
```

Uh oh. No tests. Let's write some.

In `demo_test.go`:

```
package demo

import (
        "bufio"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"
)

func TestHandleRoot_Recorder(t *testing.T) {
        rw := httptest.NewRecorder()
        handleRoot(rw, req(t, "GET / HTTP/1.0\r\n\r\n"))
        if got, want := rw.HeaderMap.Get("Content-Type"), "text/html; charset=utf-8"; got != want {
                t.Errorf("Content-Type = %q; want %q", got, want)
        }
        if !strings.Contains(rw.Body.String(), "visitor number") {
                t.Errorf("Unexpected output: %s", rw.Body)
        }
}

func req(t *testing.T, v string) *http.Request {
        req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(v)))
        if err != nil {
                t.Fatal(err)
        }
        return req
}
```

Now:

```
$ go test -v
=== RUN   TestHandleHi_Recorder
--- PASS: TestHandleHi_Recorder (0.00s)
PASS
ok      yapc/demo       0.053s

```

This tests the HTTP handler with a simple in-memory implementation of
the `ResponseWriter` interface.

Another way to write an HTTP test is to use the actual HTTP client &
server, but with automatically created localhost addresses, using the
`httptest` pacakge:

```
func TestHandleHi_TestServer(t *testing.T) {
        ts := httptest.NewServer(http.HandlerFunc(handleHi))
        defer ts.Close()
        res, err := http.Get(ts.URL)
        if err != nil {
                t.Error(err)
                return
        }
        slurp, err := ioutil.ReadAll(res.Body)
        defer res.Body.Close()
        if err != nil {
                t.Error(err)
                return
        }
        t.Logf("Got: %s", slurp)
}
```

## Race detector.

Go has concurrency built-in to the language and automatically
parallelizes code as necessary over any available CPUs. Unlike Rust,
in Go you can write code with a data race if you're not careful. A
data race is when multiple goroutine access shared data concurrently
without synchronization, when at least one of the gouroutines is doing
a write.

Before we optimize our code, let's ensure we have no data races.

Just run your tests with the `-race` flag:

```
$ go test -race
PASS
ok      yapc/demo       1.047s
```

All good, right?

Nope.

Go's race detector does runtime analysis. It has no false positives,
but it does have false negatives. If it doesn't actually see a race,
it can't report it.

Let's change our test to actually do two things at once:

```
func TestHandleHi_TestServer_Parallel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handleHi))
	defer ts.Close()
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := http.Get(ts.URL)
			if err != nil {
				t.Error(err)
				return
			}
			slurp, err := ioutil.ReadAll(res.Body)
			defer res.Body.Close()
			if err != nil {
				t.Error(err)
				return
			}
			t.Logf("Got: %s", slurp)
		}()
	}
	wg.Wait()
}
```

Now we can run it again and see:

```
$ go test -v -race
=== RUN   TestHandleHi_Recorder
--- PASS: TestHandleHi_Recorder (0.00s)
=== RUN   TestHandleHi_TestServer
--- PASS: TestHandleHi_TestServer (0.00s)
        demo_test.go:46: Got: <h1 style='color: '>Welcome!</h1>You are visitor number 2!
=== RUN   TestHandleHi_TestServer_Parallel
==================
WARNING: DATA RACE
Read by goroutine 21:
  yapc/demo.handleHi()
      /Users/bradfitz/src/yapc/demo/demo.go:17 +0xf5
  net/http.HandlerFunc.ServeHTTP()
      /Users/bradfitz/go/src/net/http/server.go:1422 +0x47
  net/http/httptest.(*waitGroupHandler).ServeHTTP()
      /Users/bradfitz/go/src/net/http/httptest/server.go:200 +0xfe
  net/http.serverHandler.ServeHTTP()
      /Users/bradfitz/go/src/net/http/server.go:1862 +0x206
  net/http.(*conn).serve()
      /Users/bradfitz/go/src/net/http/server.go:1361 +0x117c

Previous write by goroutine 23:
  yapc/demo.handleHi()
      /Users/bradfitz/src/yapc/demo/demo.go:17 +0x111
  net/http.HandlerFunc.ServeHTTP()
      /Users/bradfitz/go/src/net/http/server.go:1422 +0x47
  net/http/httptest.(*waitGroupHandler).ServeHTTP()
      /Users/bradfitz/go/src/net/http/httptest/server.go:200 +0xfe
  net/http.serverHandler.ServeHTTP()
      /Users/bradfitz/go/src/net/http/server.go:1862 +0x206
  net/http.(*conn).serve()
      /Users/bradfitz/go/src/net/http/server.go:1361 +0x117c

Goroutine 21 (running) created at:
  net/http.(*Server).Serve()
      /Users/bradfitz/go/src/net/http/server.go:1912 +0x464

Goroutine 23 (running) created at:
  net/http.(*Server).Serve()
      /Users/bradfitz/go/src/net/http/server.go:1912 +0x464
==================
--- PASS: TestHandleHi_TestServer_Parallel (0.00s)
        demo_test.go:68: Got: <h1 style='color: '>Welcome!</h1>You are visitor number 3!
        demo_test.go:68: Got: <h1 style='color: '>Welcome!</h1>You are visitor number 4!
PASS
Found 1 data race(s)
exit status 66
FAIL    yapc/demo       1.056s
```
