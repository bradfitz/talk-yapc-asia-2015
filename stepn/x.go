package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
)

var visitors int64 // must be accessed atomically

var rxOptionalID = regexp.MustCompile(`^\d*$`)

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Bad method.", http.StatusBadRequest)
		return
	}
	if !rxOptionalID.MatchString(r.FormValue("id")) {
		http.Error(w, "Optional numeric id is invalid", http.StatusBadRequest)
		return
	}
	visitNum := atomic.AddInt64(&visitors, 1)
	//io.WriteString(w, "<html><h1>Welcome!</h1>You are visitor number")
	//fmt.Fprint(w, visitNum)
	//io.WriteString(w, "!")
	fmt.Fprintf(w, "<html><h1>Welcome!</h1>You are visitor number %d!", visitNum)
}

var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32<<10)
		return &b
	},
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "Bad method; want PUT", http.StatusBadRequest)
		return
	}
	s1 := sha1.New()

	//n, err := io.Copy(s1, r.Body)

	bufp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufp)
	n, err := io.CopyBuffer(s1, r.Body, *bufp)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, "sha1 = %x in %d bytes", s1.Sum((*bufp)[:0]), n)
}

func main() {
	log.Printf("Starting on port 8080")
	http.HandleFunc("/", handleRoot)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
