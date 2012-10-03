package main

import (
	"code.google.com/p/gorilla/mux"
	"io"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"net/http"
	"time"
)

func getHandler(w http.ResponseWriter, r *http.Request) {
	uuid, ok := mux.Vars(r)["uuid"]
	if !ok {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	var entry Entry
	_, err := db.C("entry").Find(bson.M{
		"uuid": uuid,
	}).Apply(mgo.Change{
		Update: bson.M{
			"$inc": bson.M{
				"remaining_count": -1,
			},
		},
	}, &entry)
	if err != nil || entry.RemainingCount <= 0 || entry.TOD < time.Now().UnixNano() {
		Errorf(w, http.StatusNotFound, "File not found")
		return
	}

	if entry.RemainingCount <= 0 {
		db.C("entry").Remove(bson.M{
			"uuid": uuid,
		})
		Errorf(w, http.StatusNotFound, "File not found")
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
