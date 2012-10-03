package main

import (
	"code.google.com/p/gorilla/mux"
	"io"
	"labix.org/v2/mgo/bson"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"net/http"
)

func getHandler(w http.ResponseWriter, r *http.Request) {
	uuid, ok := mux.Vars(r)["uuid"]
	if !ok {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	var entry Entry
	err := db.C("entry").Find(bson.M{
		"uuid": uuid,
	}).One(&entry)
	if err != nil {
		log.Printf("db.Find failed: %s", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	bucket := s3.New(entry.Auth, aws.Regions[entry.Endpoint]).Bucket(entry.Bucket)
	rc, err := bucket.GetReader(entry.Path)
	if err != nil {
		log.Printf("bucket.GetReader failed: %s", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer rc.Close()
	io.Copy(w, rc)
}
