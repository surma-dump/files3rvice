package main

import (
	"code.google.com/p/gorilla/context"
	"code.google.com/p/gorilla/mux"
	"encoding/base64"
	"fmt"
	"github.com/surma/gouuid"
	"github.com/voxelbrain/goptions"
	"labix.org/v2/mgo"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	AWS_AUTH = iota
	S3_ENDPOINT
	S3_BUCKET
)

type Entry struct {
	Path     string      `bson:"path"`
	Endpoint string      `bson:"endpoint"`
	Auth     aws.Auth    `bson:"auth"`
	TOD      time.Time   `bson:"tod"`
	UUID     string      `bson:"uuid"`
}

var (
	options = struct {
		MongoDB string `goptions:"--mongodb, obligatory, description='MongoDB to connect to'"`
		Listen  string `goptions:"--listen, -l, description='Address to bind HTTP server to (default: localhost:8080)'"`
		BaseURL string `goptions:"--base-url, -b, description='URL prefix'"`
	}{
		Listen:  "localhost:8080",
		BaseURL: "http://localhost",
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

	static := r.PathPrefix("/").Subrouter()
	static.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	err = http.ListenAndServe(options.Listen, r)
	if err != nil {
		log.Fatalf("Failed to start webserver: %s", err)
	}
}

func awsAuthContext(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auths, ok := r.Header["Authorization"]
		if !ok || len(auths) < 1 {
			http.Error(w, "No authorization given", http.StatusNotFound)
			return
		}
		auth := auths[0]

		if !strings.HasPrefix(auth, "Basic") {
			http.Error(w, "Invalid authorization type", http.StatusNotFound)
			return
		}
		auth = auth[len("Basic "):]

		plainauth, err := base64.StdEncoding.DecodeString(auth)
		if err != nil {
			http.Error(w, "Invalid authorization data", http.StatusNotFound)
			return
		}

		credentials := strings.Split(string(plainauth), ":")
		if len(credentials) != 2 {
			http.Error(w, "Invalid authorization data", http.StatusNotFound)
			return
		}

		aws_auth := aws.Auth{
			AccessKey: credentials[0],
			SecretKey: credentials[1],
		}
		context.DefaultContext.Set(r, AWS_AUTH, aws_auth)
		f(w, r)
	}
}

func s3BucketContext(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		aws_auth := context.DefaultContext.Get(r, AWS_AUTH).(aws.Auth)
		bucketurls, ok := r.Header["X-Bucket"]
		if !ok || len(bucketurls) < 1 {
			http.Error(w, "Missing bucket address", http.StatusNotFound)
			return
		}

		bucketname, endpointname, ok := splitBucketURL(bucketurls[0])
		if !ok {
			http.Error(w, "Invalid bucket address", http.StatusNotFound)
			return
		}

		aws_endpoint, ok := aws.Regions[endpointname]
		if !ok {
			http.Error(w, "Invalid endpoint name", http.StatusNotFound)
			return
		}

		context.DefaultContext.Set(r, S3_ENDPOINT, aws_endpoint)
		bucket := s3.New(aws_auth, aws_endpoint).Bucket(bucketname)
		context.DefaultContext.Set(r, S3_BUCKET, bucket)
		f(w, r)
	}
}

func splitBucketURL(url string) (bucketname string, endpoint string, ok bool) {
	ok = true
	parts := strings.Split(url, ".")
	if len(parts) <= 3 {
		ok = false
		return
	}
	bucketname = strings.Join(parts[0:len(parts)-3], ".")
	endpointurl := parts[len(parts)-3]
	endpointparts := strings.Split(endpointurl, "-")
	if len(endpointparts) != 5 {
		ok = false
		return
	}
	endpoint = strings.Join(endpointparts[2:], "-")
	return
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	path, ok := mux.Vars(r)["path"]
	if !ok {
		http.Error(w, "No path supplied", http.StatusNotFound)
		return
	}

	entry := Entry{
		Path:     path,
		Endpoint: context.DefaultContext.Get(r, S3_ENDPOINT).(aws.Region).Name,
		Auth:     context.DefaultContext.Get(r, AWS_AUTH).(aws.Auth),
		TOD:      time.Now().Add(24 * 7 * time.Hour),
		UUID:     gouuid.New().String(),
	}
	err := db.C("entry").Insert(entry)
	if err != nil {
		log.Printf("db.Insert failed: %s", err)
		http.Error(w, "Could not save to S3", http.StatusNotFound)
		return
	}

	bucket := context.DefaultContext.Get(r, S3_BUCKET).(*s3.Bucket)
	err = bucket.PutReader(path, r.Body, r.ContentLength, mime.TypeByExtension(filepath.Ext(path)), s3.Private)
	if err != nil {
		log.Printf("bucket.PutReader failed: %s", err)
		http.Error(w, "Could not push to S3", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%s/api/get/%s\n", options.BaseURL, entry.UUID)

}
