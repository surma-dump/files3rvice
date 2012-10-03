package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

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

func Errorf(w http.ResponseWriter, code int, format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Println(str)
	http.Error(w, str, code)
}
