# Profiling & Optimizing in Go

Brad Fitzpatrick

YAPC::Asia 2015

Tokyo Big Sight, 2015-08-22

## Requirements

If you're following along at home, you'll need the following:

* Go (1.4.2 or 1.5+ recommended)
* Graphviz (http://www.graphviz.org/)
* Linux (ideal), Windows, or OS X (requires http://godoc.org/rsc.io/pprof_mac_fix)

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

Now we can see that the write on line 17 (to the `visitors` variable)
conflicts with the read on line 17 (of the same variable). To make it
more obvious, change the code to:

```
    now := visitors + 1
    visitors = now
```

... and it'll report different line numbers for each.

## Fix the race!

If your code has data races, all bets are off and you're just waiting
for a crash. The runtime promises nothing if you have a data race.

Multiple options:

* use channels ("share by communication, don't communication by sharing")
* use a Mutex
* use atomic

### Mutex

```
  var visitors struct {
    sync.Mutex
    n int
  }
...
  func foo() {
    ...
    visitors.Lock()
    visitors.n++
    yourVisitorNumber := visitors.n
    visitors.Unlock()
```

### Atomic

```
  var visitors int64 // must be accessed atomically
...
  func foo() {
    ...
    visitNum := atomic.AddInt64(&visitors, 1)
```

## How fast can it go? CPU Profiling!

To use Go's CPU profiling, it's easiest to first write a `Benchmark`
function, which is very similar to a `Test` function.

```
func BenchmarkHi(b *testing.B) {
        r := req(b, "GET / HTTP/1.0\r\n\r\n")
        for i := 0; i < b.N; i++ {
                rw := httptest.NewRecorder()
                handleHi(rw, r)
        }
}
```

(and change `func req` to take the `testing.TB` interface instead, so
it can take a `*testing.T` or a `*testing.B`)

Now we can run the benchmarks:

```
$ go test -v -run=^$ -bench=. 
PASS
BenchmarkHi-4     100000             12843 ns/op
ok      yapc/demo       1.472s
```

Play with flags, like `-benchtime`.

Is that fast? Slow? Your decision.

But let's see where the CPU is going now....

## CPU Profiling

```
$ go test -v -run=^$ -bench=^BenchmarkHi$ -benchtime=2s -cpuprofile=prof.cpu
```

(Leaves `demo.test` binary behind)

Now, let's use the Go profile viewer:

```
$ go tool pprof demo.test prof.cpu
Entering interactive mode (type "help" for commands)

(pprof) top 
3070ms of 3850ms total (79.74%)
Dropped 62 nodes (cum <= 19.25ms)
Showing top 10 nodes out of 92 (cum >= 290ms)
      flat  flat%   sum%        cum   cum%
    1710ms 44.42% 44.42%     1710ms 44.42%  runtime.mach_semaphore_signal
     290ms  7.53% 51.95%     1970ms 51.17%  runtime.growslice
     230ms  5.97% 57.92%      230ms  5.97%  runtime.mach_semaphore_wait
     200ms  5.19% 63.12%     2270ms 58.96%  runtime.mallocgc
     160ms  4.16% 67.27%      160ms  4.16%  runtime.heapBitsSetType
     110ms  2.86% 70.13%      210ms  5.45%  runtime.mapassign1
     110ms  2.86% 72.99%      110ms  2.86%  runtime.memclr
     100ms  2.60% 75.58%      640ms 16.62%  regexp.makeOnePass.func2
     100ms  2.60% 78.18%      100ms  2.60%  runtime.memmove
      60ms  1.56% 79.74%      290ms  7.53%  runtime.makeslice

(pprof) top --cum
0.26s of 3.85s total ( 6.75%)
Dropped 62 nodes (cum <= 0.02s)
Showing top 10 nodes out of 92 (cum >= 2.22s)
      flat  flat%   sum%        cum   cum%
         0     0%     0%      3.55s 92.21%  runtime.goexit
         0     0%     0%      3.48s 90.39%  testing.(*B).launch
         0     0%     0%      3.48s 90.39%  testing.(*B).runN
     0.01s  0.26%  0.26%      3.47s 90.13%  yapc/demo.BenchmarkHi
     0.01s  0.26%  0.52%      3.44s 89.35%  yapc/demo.handleHi
         0     0%  0.52%      3.30s 85.71%  regexp.MatchString
     0.01s  0.26%  0.78%         3s 77.92%  regexp.Compile
         0     0%  0.78%      2.99s 77.66%  regexp.compile
     0.20s  5.19%  5.97%      2.27s 58.96%  runtime.mallocgc
     0.03s  0.78%  6.75%      2.22s 57.66%  regexp.compileOnePass

(pprof) list handleHi
Total: 3.85s
ROUTINE ======================== yapc/demo.handleHi in /Users/bradfitz/src/yapc/demo/demo.go
      10ms      3.44s (flat, cum) 89.35% of Total
         .          .      8:)
         .          .      9:
         .          .     10:var visitors int
         .          .     11:
         .          .     12:func handleHi(w http.ResponseWriter, r *http.Request) {
         .      3.30s     13:   if match, _ := regexp.MatchString(\w*$r.FormValue("color")); !match {
         .          .     14:           http.Error(w, "Optional color is invalid", http.StatusBadRequest)
         .          .     15:           return
         .          .     16:   }
      10ms       10ms     17:   visitors++
         .       50ms     18:   w.Header().Set("Content-Type", "text/html; charset=utf-8")
         .       80ms     19:   w.Write([]byte("<h1 style='color: " + r.FormValue("color") + "'>Welcome!</h1>You are visitor number " + fmt.Sprint(visitors) + "!"))
         .          .     20:}
         .          .     21:
         .          .     22:func main() {
         .          .     23:   log.Printf("Starting on port 8080")
         .          .     24:   http.HandleFunc("/hi", handleHi)

(pprof) web
```

![cpu0.svg](cpu0.svg)

