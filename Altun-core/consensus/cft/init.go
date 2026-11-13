package cft

import (
	"flag"
	"net/http"

	"github.com/wtwinlab/altun/lib/log"
)

// Init setup cft package
func Init() {
	flag.Parse()
	log.Setup()
	config.Load()
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 1000
}
