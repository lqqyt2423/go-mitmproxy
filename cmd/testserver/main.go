package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

var cert string = os.Getenv("SERVER_CERT_FILE")
var key string = os.Getenv("SERVER_KEY_FILE")
var httpAddr string = ":8080"
var httpsAddr string = ":8443"

type Server struct{}

func (server *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.Printf("%v %v", req.Method, req.URL.String())
	_, _ = rw.Write([]byte("hello world\n"))
}

func main() {
	go func() {
		server := &http.Server{
			Addr:    httpAddr,
			Handler: &Server{},
		}
		log.Printf("http server listen at %v\n", httpAddr)
		log.Fatal(server.ListenAndServe())
	}()

	server := &http.Server{
		Addr:    httpsAddr,
		Handler: &Server{},
	}
	log.Printf("https server listen at %v\n", httpsAddr)
	log.Fatal(server.ListenAndServeTLS(cert, key))
}
