package api

import (
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/korylprince/competition-scorer/db"
)

//NewRouter returns an HTTP router for the HTTP API
func NewRouter(db db.DB, sess *MemorySessionStore, sub *SubscribeService) http.Handler {

	r := mux.NewRouter()

	r.Path("/auth").Methods("POST").Handler(postAuth(db, sess))
	r.Path("/auth").Methods("PUT").Handler(putAuth(db, sess))
	r.Path("/competition").Methods("GET").Handler(getCompetition(db))
	r.Path("/competition").Methods("PUT").Handler(putCompetition(db, sess, sub))
	r.Path("/competition/subscribe").Handler(subscribeCompetition(sub))
	r.Path("/competition/revisions").Methods("GET").Handler(getRevisions(db, sess))
	r.Path("/competition/revisions/{id:[0-9]+}").Methods("GET").Handler(getRevision(db, sess))

	r.NotFoundHandler = http.HandlerFunc(notFound)

	chain := handlers.LoggingHandler(os.Stdout, handlers.CompressHandler(handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Accept", "Authorization", "Content-Type", "Origin"}),
	)(http.StripPrefix("/api/1.0", r))))

	return chain
}
