package api

import (
	"encoding/json"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/korylprince/competition-scorer/db"
)

var authRegexp = regexp.MustCompile("^SESSION id=([a-zA-Z0-9]{22})$")

type jsonError struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

func codeToJSON(code int) *jsonError {
	return &jsonError{Code: code, Description: http.StatusText(code)}
}

//returnHTTP writes the correct headers. If body is not nil then it's JSON encoded.
//Otherwise a JSON representation of the HTTP code is encoded
func returnHTTP(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if body == nil {
		body = codeToJSON(code)
	}

	e := json.NewEncoder(w)
	err := e.Encode(body)
	if err != nil {
		log.Println("Unable to encode body:", err)
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	returnHTTP(w, http.StatusNotFound, nil)
}

func checkJSON(w http.ResponseWriter, r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		returnHTTP(w, http.StatusBadRequest, nil)
		return false
	}

	if mediaType != "application/json" {
		returnHTTP(w, http.StatusBadRequest, nil)
		return false
	}

	return true
}

//checkAuth checks if the given request is authorized in the session store
//If the request is not authorized checkAuth returns false and writes the error to w
//Otherwise checkAuth returns true
func checkAuth(w http.ResponseWriter, r *http.Request, s *MemorySessionStore) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		returnHTTP(w, http.StatusUnauthorized, nil)
		return false
	}

	match := authRegexp.FindStringSubmatch(auth)
	if len(match) != 2 {
		returnHTTP(w, http.StatusBadRequest, nil)
		return false
	}

	id := match[1]
	if !s.Check(id) {
		returnHTTP(w, http.StatusUnauthorized, nil)
		return false
	}

	return true
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	SessionID string `json:"session_id"`
}

func postAuth(d db.DB, s *MemorySessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkJSON(w, r) {
			return
		}

		a := new(authRequest)
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(a); err != nil {
			log.Println("Unable to decode request body:", err)
			returnHTTP(w, http.StatusBadRequest, nil)
			return
		}

		status, err := d.Authenticate(a.Username, a.Password)
		if err != nil {
			log.Println("Unable to check username/password:", err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		if !status {
			returnHTTP(w, http.StatusUnauthorized, nil)
			return
		}

		returnHTTP(w, http.StatusOK, &authResponse{SessionID: s.Create()})
	}
}

func putAuth(d db.DB, s *MemorySessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkJSON(w, r) {
			return
		}

		if !checkAuth(w, r, s) {
			return
		}

		c := new(authRequest)
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(c); err != nil {
			log.Println("Unable to decode request body:", err)
			returnHTTP(w, http.StatusBadRequest, nil)
			return
		}

		err := d.UpdateCredentials(c.Username, c.Password)
		if err != nil {
			log.Println("Unable to update credentials:", err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		returnHTTP(w, http.StatusOK, nil)
	}
}

func getCompetition(d db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := d.Read()
		if err != nil {
			log.Println("Unable to read database:", err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		if c == nil {
			returnHTTP(w, http.StatusNotFound, nil)
			return
		}

		returnHTTP(w, http.StatusOK, c)
	}
}

type createRequest struct {
	Name     string   `json:"name"`
	Rounds   int      `json:"rounds"`
	Teams    []string `json:"teams"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

type createResponse struct {
	Competition *db.Competition `json:"competition"`
	SessionID   string          `json:"session_id"`
}

func createCompetition(w http.ResponseWriter, r *http.Request, d db.DB, s *MemorySessionStore) {
	c := new(createRequest)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(c); err != nil {
		log.Println("Unable to decode request body:", err)
		returnHTTP(w, http.StatusBadRequest, nil)
		return
	}

	err := d.Init(c.Name, c.Rounds, c.Teams, c.Username, c.Password)
	if err != nil {
		log.Println("Unable to init database:", err)
		returnHTTP(w, http.StatusInternalServerError, nil)
		return
	}

	comp, err := d.Read()
	if err != nil {
		log.Println("Unable to read database:", err)
		returnHTTP(w, http.StatusInternalServerError, nil)
		return
	}

	returnHTTP(w, http.StatusCreated, &createResponse{Competition: comp, SessionID: s.Create()})
}

type putRequest struct {
	Competition *db.Competition `json:"competition"`
	ID          int             `json:"id"`
}

func putCompetition(d db.DB, sess *MemorySessionStore, sub *SubscribeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkJSON(w, r) {
			return
		}

		oldComp, err := d.Read()
		if err != nil {
			log.Println("Unable to read database:", err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		if oldComp == nil {
			createCompetition(w, r, d, sess)
			return
		}

		if !checkAuth(w, r, sess) {
			return
		}

		req := new(putRequest)
		dec := json.NewDecoder(r.Body)
		if err = dec.Decode(req); err != nil {
			log.Println("Unable to decode request body:", err)
			returnHTTP(w, http.StatusBadRequest, nil)
			return
		}

		err = d.Write(req.Competition)
		if err != nil {
			log.Println("Unable to write database:", err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		returnHTTP(w, http.StatusOK, nil)
		sub.Notify(req.ID)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type subscribeResponse struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

func subscribeCompetition(s *SubscribeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Unable to start WebSocket connection:", err)
			return
		}

		myID, sub := s.Subscribe()
		err = conn.WriteJSON(subscribeResponse{Type: "connect", ID: myID})
		if err != nil {
			log.Println("Unable to write WebSocket message:", err)
			s.Unsubscribe(myID)
			return
		}

		for {
			id := <-sub
			err = conn.WriteJSON(subscribeResponse{Type: "update", ID: id})
			if err != nil {
				log.Println("Unable to write WebSocket message:", err)
				s.Unsubscribe(myID)
				return
			}
		}
	}
}

type revisionsRequest struct {
	Revisions []*db.Revision `json:"revisions"`
}

func getRevisions(d db.DB, s *MemorySessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, s) {
			return
		}

		rev, err := d.Revisions()
		if err != nil {
			log.Println("Unable to read database revisions:", err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		returnHTTP(w, http.StatusOK, &revisionsRequest{Revisions: rev})
	}
}

func getRevision(d db.DB, s *MemorySessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, s) {
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			returnHTTP(w, http.StatusBadRequest, nil)
			return
		}

		rev, err := d.ReadRevision(int32(id))
		if err != nil {
			log.Printf("Unable to read database revision %d: %v", id, err)
			returnHTTP(w, http.StatusInternalServerError, nil)
			return
		}

		returnHTTP(w, http.StatusOK, rev)
	}
}
