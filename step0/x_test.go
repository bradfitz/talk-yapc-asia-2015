package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func req(t testing.TB, v string) *http.Request {
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(v)))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

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

func TestHandleRoot_TestServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handleRoot))
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

func TestHandleRoot_TestServer_Parallel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(handleRoot))
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

func BenchmarkRoot(b *testing.B) {
	r := req(b, "GET / HTTP/1.0\r\n\r\n")
	for i := 0; i < b.N; i++ {
		rw := httptest.NewRecorder()
		handleRoot(rw, r)
	}
}

func BenchmarkConcat(b *testing.B) {
	benchmarkHandler(b, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("You are visitor number " + strconv.Itoa(1) + "!"))
	})
}

func BenchmarkSprint(b *testing.B) {
	benchmarkHandler(b, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("You are visitor number " + fmt.Sprint(1) + "!"))
	})
}

func BenchmarkFprintf(b *testing.B) {
	benchmarkHandler(b, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You are visitor number %d!", 1)
	})
}

func BenchmarkSyncPool(b *testing.B) {
	p := &sync.Pool{
		New: func() interface{} {
			b := make([]byte, 1024)
			return &b
		},
	}
	benchmarkHandler(b, func(w http.ResponseWriter, r *http.Request) {
		bufp := p.Get().(*[]byte)
		defer p.Put(bufp)
		buf := (*bufp)[:0]
		buf = append(buf, "You are visitor number "...)
		buf = strconv.AppendInt(buf, 1, 10)
		buf = append(buf, '!')
		w.Write(buf)
	})
}

func benchmarkHandler(b *testing.B, fn http.HandlerFunc) {
	b.ReportAllocs()
	r := req(b, "GET / HTTP/1.0\r\n\r\n")
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			fn(new(httptest.ResponseRecorder), r)
		}
	})
}
