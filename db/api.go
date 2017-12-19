package db

import "time"

//Team represents a competition team
type Team struct {
	Name   string   `json:"name"`
	Scores []*int32 `json:"scores"`
}

//Competition represents a competition
type Competition struct {
	Name   string   `json:"name"`
	Rounds []string `json:"rounds"`
	Teams  []*Team  `json:"teams"`
}

//Revision represents a revision of a competition
type Revision struct {
	ID          int32        `json:"id"`
	Timestamp   time.Time    `json:"timestamp"`
	Competition *Competition `json:"competition,omitempty"`
}

//DB is a competition database
type DB interface {
	//Init initializes the database with the given parameters
	Init(name string, rounds int, teams []string, username, password string) error

	//Authenticate returns if the given username and password is correct or an error if one occurred
	Authenticate(username, password string) (status bool, err error)

	//UpdateCredentials updates the database with the given username and password or returns an error if one occurred
	UpdateCredentials(username, password string) error

	//Revisions returns all of the revisions in the database or an error if one occurred.
	//Note: Competition will be nil
	Revisions() ([]*Revision, error)

	//ReadRevisions returns the Revision with the given id or an error if one occurred
	//If the revision with the given id doesn't exist, ReadRevision will return nil
	ReadRevision(id int32) (*Revision, error)

	//Read returns the Competition stored in the database or an error if one occurred.
	//Read returns a nil Competition if the database is empty
	Read() (*Competition, error)

	//Write stores the given Competition in the database or an error if one occurred.
	//Write clears the database if Competition is nil
	Write(c *Competition) error
}
