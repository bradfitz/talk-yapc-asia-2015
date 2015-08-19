package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHandleRoot(t *testing.T) {
	rw := httptest.NewRecorder()
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader("GET / HTTP/1.0\r\n\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	handleRoot(rw, req)
	t.Logf("Got: %#v", rw)
	t.Logf("Out: %s", rw.Body)
}

type neverEnding byte

func (b neverEnding) Read(p []byte) (n int, err error) {
	if len(p) < 16 {
		for i := range p {
			p[i] = byte(b)
		}
	} else {
		b.Read(p[:len(p)/2])
		copy(p[len(p)/2:], p)
	}
	return len(p), nil
}

func BenchmarkNeverending(b *testing.B) {
	buf := make([]byte, 4096)
	A := neverEnding('A')
	for i := 0; i < b.N; i++ {
		A.Read(buf)
	}
}

func BenchmarkPut(b *testing.B) {
	b.ReportAllocs()
	const length = 64 << 10
	b.SetBytes(length)
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader("PUT / HTTP/1.1\r\n" +
		"Content-Type: application/x-something\r\n" +
		"Content-Length: " + strconv.Itoa(length) + "\r\n" +
		"\r\n")))
	if err != nil {
		b.Fatal(err)
	}
	rw := httptest.NewRecorder()
	lr := io.LimitReader(neverEnding('a'), length)
	body := ioutil.NopCloser(lr)
	for i := 0; i < b.N; i++ {
		rw.Body.Reset()
		lr.(*io.LimitedReader).N = length
		req.Body = body
		handlePost(rw, req)
	}
}
