package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"github.com/voxelbrain/goptions"
	"labix.org/v2/mgo"
	"launchpad.net/goamz/aws"
	"log"
	"net/http"
)

const (
	AWS_AUTH = iota
	S3_ENDPOINT
	S3_BUCKET
	HEADER_TOD
	HEADER_MAX_ACCESS
)

type Entry struct {
	UUID           string   `bson:"uuid"`
	Endpoint       string   `bson:"endpoint"`
	Bucket         string   `bson:"bucket"`
	Path           string   `bson:"path"`
	Auth           aws.Auth `bson:"auth"`
	TOD            int64    `bson:"tod"`
	RemainingCount int64    `bson:"remaining_count"`
}

var (
	options = struct {
		MongoDB string `goptions:"--mongodb, obligatory, description='MongoDB to connect to'"`
		Listen  string `goptions:"--listen, -l, description='Address to bind HTTP server to (default: localhost:8080)'"`
	}{
		Listen: "localhost:8080",
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
	api.Methods("POST").Path("/upload/{path:.+}").HandlerFunc(
		cleanupWrapper(
			maxAccessContext(
				ttlContext(
					awsAuthContext(
						s3BucketContext(
							uploadHandler))))))
	api.HandleFunc("/get/{uuid:[0-9a-f-]+}", cleanupWrapper(getHandler))

	static := r.PathPrefix("/").Subrouter()
	static.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	log.Println("Starting server...")
	err = http.ListenAndServe(options.Listen, r)
	if err != nil {
		log.Fatalf("Failed to start webserver: %s", err)
	}
}
