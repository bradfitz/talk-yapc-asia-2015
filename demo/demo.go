package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
)

var visitors int

var colorRx = regexp.MustCompile(`^\w*$`)

func handleHi(w http.ResponseWriter, r *http.Request) {
	if !colorRx.MatchString(r.FormValue("color")) {
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
