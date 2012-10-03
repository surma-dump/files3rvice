package main

import (
	"code.google.com/p/gorilla/context"
	"encoding/base64"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
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

func ttlContext(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ttl_string, ok := r.Header["X-Ttl"]
		if !ok || len(ttl_string) < 1 {
			log.Printf("Using default TTL (%v, %#v)", ok, ttl_string)
			ttl_string = []string{"-1"}
		}

		ttl, err := strconv.ParseInt(ttl_string[0], 10, 64)
		if err != nil {
			http.Error(w, "Invalid TTL", http.StatusNotFound)
			return
		}
		if ttl != -1 {
			ttl = time.Now().Add(time.Duration(ttl) * time.Minute).UnixNano()
		}

		context.DefaultContext.Set(r, HEADER_TOD, ttl)
		f(w, r)
	}
}

func maxAccessContext(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ma_string, ok := r.Header["X-Max-Access"]
		if !ok || len(ma_string) < 1 {
			ma_string = []string{"-1"}
		}

		ttl, err := strconv.ParseInt(ma_string[0], 10, 64)
		if err != nil {
			http.Error(w, "Invalid Max-Access", http.StatusNotFound)
			return
		}

		context.DefaultContext.Set(r, HEADER_MAX_ACCESS, ttl)
		f(w, r)
	}
}

func cleanupWrapper(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		for err == nil {
			var entry Entry
			_, err = db.C("entry").Find(bson.M{
				"$or": []bson.M{
					bson.M{
						"$and": []bson.M{
							bson.M{
								"tod": bson.M{
									"$lt": time.Now().UnixNano(),
								},
							},
							bson.M{
								"tod": bson.M{
									"$gte": 0,
								},
							},
						},
					},
					bson.M{
						"remaining_count": bson.M{
							"$lte": 0,
						},
					},
				},
			}).Apply(mgo.Change{
				Remove: true,
			}, &entry)

			if err != nil {
				continue
			}

			bucket := s3.New(entry.Auth, aws.Regions[entry.Endpoint]).Bucket(entry.Bucket)
			err := bucket.Del(entry.Path)
			if err != nil {
				log.Printf("cleanupWrapper: S3 delete failed: %s", err)
			}
		}
		f(w, r)
	}
}
