package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"github.com/voxelbrain/goptions"
	"labix.org/v2/mgo"
	"launchpad.net/goamz/aws"
	"log"
	"net/http"
	"time"
)

const (
	AWS_AUTH = iota
	S3_ENDPOINT
	S3_BUCKET
)

type Entry struct {
	Bucket   string    `bson:"bucket"`
	Path     string    `bson:"path"`
	Endpoint string    `bson:"endpoint"`
	Auth     aws.Auth  `bson:"auth"`
	TOD      time.Time `bson:"tod"`
	UUID     string    `bson:"uuid"`
}

var (
	options = struct {
		MongoDB string `goptions:"--mongodb, obligatory, description='MongoDB to connect to'"`
		Listen  string `goptions:"--listen, -l, description='Address to bind HTTP server to (default: localhost:8080)'"`
		BaseURL string `goptions:"--base-url, -b, description='URL prefix'"`
	}{
		Listen:  "localhost:8080",
		BaseURL: "http://localhost:8080",
	}
	db *mgo.Database
)

func main() {
	err := goptions.Parse(&options)
	if err != nil {
		if err != goptions.ErrHelpRequest {
			fmt.Printf("Error: %s\n", err)
		}
		goptions.PrintHelp()
		return
	}

	session, err := mgo.Dial(options.MongoDB)
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %s", err)
	}
	defer session.Close()
	db = session.DB("files3rvice")

	r := mux.NewRouter()
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/upload/{path:.+}", awsAuthContext(s3BucketContext(uploadHandler)))
	api.HandleFunc("/get/{uuid:[0-9a-f-]+}", getHandler)

	static := r.PathPrefix("/").Subrouter()
	static.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	err = http.ListenAndServe(options.Listen, r)
	if err != nil {
		log.Fatalf("Failed to start webserver: %s", err)
	}
}
