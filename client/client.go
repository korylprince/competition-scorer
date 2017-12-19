package client

import "net/http"

//Handler is an http.Handler for the client filesystem
var Handler = http.FileServer(assetFS())
