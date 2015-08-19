package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
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

func BenchmarkRoot(b *testing.B) {
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader("GET / HTTP/1.0\r\n\r\n")))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		rw := httptest.NewRecorder()
		handleRoot(rw, req)
	}
}
