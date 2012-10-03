package main

import (
	"code.google.com/p/gorilla/context"
	"encoding/base64"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"net/http"
	"strings"
)

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
