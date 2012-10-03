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
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	path, ok := mux.Vars(r)["path"]
	if !ok {
		http.Error(w, "No path supplied", http.StatusNotFound)
		return
	}

	entry := Entry{
		UUID:           gouuid.New().String(),
		Endpoint:       context.DefaultContext.Get(r, S3_ENDPOINT).(aws.Region).Name,
		Bucket:         context.DefaultContext.Get(r, S3_BUCKET).(*s3.Bucket).Name,
		Path:           path,
		Auth:           context.DefaultContext.Get(r, AWS_AUTH).(aws.Auth),
		TOD:            context.DefaultContext.Get(r, HEADER_TOD).(int64),
		RemainingCount: context.DefaultContext.Get(r, HEADER_MAX_ACCESS).(int64),
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
	fmt.Fprintf(w, "%s\n", entry.UUID)

}
