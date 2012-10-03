package main

import (
	"code.google.com/p/gorilla/context"
	"code.google.com/p/gorilla/mux"
	"fmt"
	"github.com/surma/gouuid"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"time"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	path, ok := mux.Vars(r)["path"]
	if !ok {
		http.Error(w, "No path supplied", http.StatusNotFound)
		return
	}

	entry := Entry{
		Path:     path,
		Bucket:   context.DefaultContext.Get(r, S3_BUCKET).(*s3.Bucket).Name,
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
