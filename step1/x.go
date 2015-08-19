package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
)

var visitors int

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
	visitors++
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<h1>Welcome!</h1>You are visitor number " + fmt.Sprint(visitors) + "!"))
}

func main() {
	log.Printf("Starting on port 8080")
	http.HandleFunc("/", handleRoot)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
