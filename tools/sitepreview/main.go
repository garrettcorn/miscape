// Command sitepreview serves site/ locally for previewing the website:
//
//	go run ./tools/sitepreview   (PORT, default 8150)
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8150"
	}
	log.Printf("previewing site/ at http://127.0.0.1:%s/", port)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+port, http.FileServer(http.Dir("site"))))
}
