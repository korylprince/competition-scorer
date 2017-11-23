package db

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	"golang.org/x/crypto/bcrypt"
)

type boltDB struct {
	*bolt.DB
}

//New returns a new DB with the given file path
func New(path string) (DB, error) {
	db, err := bolt.Open(path, 0644, nil)
	return &boltDB{db}, err
}

func (db *boltDB) Init(name string, rounds int, teams []string, username, password string) error {
	if err := db.UpdateCredentials(username, password); err != nil {
		return &Error{Err: err, Description: "Couldn't update credentials"}
	}

	c := &Competition{
		Name:   name,
		Rounds: make([]string, 0, rounds),
		Teams:  make([]*Team, 0, len(teams)),
	}

	for r := 1; r <= rounds; r++ {
		c.Rounds = append(c.Rounds, fmt.Sprintf("Round %d", r))
	}

	for _, team := range teams {
		t := &Team{
			Name:   team,
			Scores: make([]int32, rounds),
		}

		c.Teams = append(c.Teams, t)
	}

	if err := db.Write(c); err != nil {
		return &Error{Err: err, Description: "Couldn't write competition to database"}
	}

	return nil
}

func (db *boltDB) Authenticate(username string, password string) (status bool, err error) {
	tx, err := db.Begin(false)
	if err != nil {
		return false, &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		lErr := tx.Rollback()
		if err == nil && lErr != nil {
			err = &Error{Err: err, Description: "Couldn't end transaction"}
		}
	}()

	configBucket := tx.Bucket([]byte("config"))
	if configBucket == nil {
		return false, &Error{Err: nil, Description: "Database config Bucket was nil"}
	}

	if username != string(configBucket.Get([]byte("username"))) {
		return false, nil
	}

	hash := configBucket.Get([]byte("hash"))
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil, nil
}

func (db *boltDB) UpdateCredentials(username string, password string) (err error) {
	tx, err := db.Begin(true)
	if err != nil {
		return &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		if err != nil {
			lErr := tx.Rollback()
			if lErr != nil {
				err = &Error{Err: lErr, Description: fmt.Sprintf("Couldn't rollback transaction; error causing rollback: %s", err)}
			}
			return
		}
		lErr := tx.Commit()
		if lErr != nil {
			err = &Error{Err: err, Description: "Couldn't commit transaction"}
		}
	}()

	configBucket, err := tx.CreateBucketIfNotExists([]byte("config"))
	if err != nil {
		return &Error{Err: err, Description: "Couldn't create Database config Bucket"}
	}

	if err = configBucket.Put([]byte("username"), []byte(username)); err != nil {
		return &Error{Err: err, Description: "Couldn't update username"}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return &Error{Err: err, Description: "Couldn't hash password"}
	}

	if err = configBucket.Put([]byte("hash"), hash); err != nil {
		return &Error{Err: err, Description: "Couldn't update password hash"}
	}

	return nil
}

func (db *boltDB) LastModified() (t time.Time, err error) {
	tx, err := db.Begin(false)
	if err != nil {
		return t, &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		lErr := tx.Rollback()
		if err == nil && lErr != nil {
			err = &Error{Err: err, Description: "Couldn't end transaction"}
		}
	}()

	competitionBucket := tx.Bucket([]byte("competition"))
	if competitionBucket == nil {
		return t, &Error{Err: nil, Description: "Database competition Bucket was nil"}
	}

	configBucket := competitionBucket.Bucket([]byte("config"))
	if configBucket == nil {
		return t, &Error{Err: nil, Description: "Competition config Bucket was nil"}
	}

	err = t.UnmarshalBinary(configBucket.Get([]byte("last_modified")))
	if err != nil {
		return t, &Error{Err: err, Description: "Couldn't decode last_modified"}
	}

	return t, nil
}

func (db *boltDB) getLatestRevision(tx *bolt.Tx) (int32, error) {
	configBucket := tx.Bucket([]byte("config"))
	if configBucket == nil {
		return 0, &Error{Err: nil, Description: "Database config Bucket was nil"}
	}

	last := configBucket.Get([]byte("current_revision"))
	if last == nil {
		return -1, nil
	}

	return bytesToInt(last)
}

func (db *boltDB) Revisions() ([]*Revision, error) {
	tx, err := db.Begin(false)
	if err != nil {
		return nil, &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		lErr := tx.Rollback()
		if err == nil && lErr != nil {
			err = &Error{Err: err, Description: "Couldn't end transaction"}
		}
	}()

	revisionsBucket := tx.Bucket([]byte("revisions"))
	if revisionsBucket == nil {
		return nil, nil
	}

	last, err := db.getLatestRevision(tx)
	if err != nil {
		return nil, &Error{Err: err, Description: "Couldn't get latest Revision"}
	}

	revisions := make([]*Revision, 0, last+1)

	for i := 0; i <= int(last); i++ {
		revisionBucket := revisionsBucket.Bucket(intToBytes(int32(i)))
		if revisionBucket == nil {
			return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't get Revision(%d)", i)}
		}

		configBucket := revisionBucket.Bucket([]byte("config"))
		if configBucket == nil {
			return nil, &Error{Err: err, Description: fmt.Sprintf("Revision(%d) config Bucket was nil", i)}
		}

		lastModified := configBucket.Get([]byte("last_modified"))

		var t time.Time
		err = t.UnmarshalBinary(lastModified)
		if err != nil {
			return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't decode Revision(%d) config.last_modified(%#v)", i, lastModified)}
		}

		revisions = append(revisions, &Revision{ID: int32(i), Timestamp: t})
	}

	return revisions, nil
}

func (db *boltDB) ReadRevision(id int32) (*Revision, error) {
	tx, err := db.Begin(false)
	if err != nil {
		return nil, &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		lErr := tx.Rollback()
		if err == nil && lErr != nil {
			err = &Error{Err: err, Description: "Couldn't end transaction"}
		}
	}()

	revisionsBucket := tx.Bucket([]byte("revisions"))
	if revisionsBucket == nil {
		return nil, nil
	}

	revisionBucket := revisionsBucket.Bucket(intToBytes(id))
	if revisionBucket == nil {
		return nil, nil
	}

	configBucket := revisionBucket.Bucket([]byte("config"))
	if configBucket == nil {
		return nil, &Error{Err: nil, Description: fmt.Sprintf("Revision(%d) config Bucket was nil", id)}
	}

	lastModified := configBucket.Get([]byte("last_modified"))

	var t time.Time
	err = t.UnmarshalBinary(lastModified)
	if err != nil {
		return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't decode Revision(%d) config.last_modified(%#v)", id, lastModified)}
	}

	competitionBucket := revisionBucket.Bucket([]byte("competition"))
	if competitionBucket == nil {
		return nil, &Error{Err: nil, Description: fmt.Sprintf("Revision(%d) competition Bucket was nil", id)}
	}

	c, err := readCompetition(competitionBucket)
	if err != nil {
		return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't read Revision(%d) Competition", id)}
	}

	return &Revision{ID: id, Timestamp: t, Competition: c}, nil
}

func (db *boltDB) Read() (c *Competition, err error) {
	tx, err := db.Begin(false)
	if err != nil {
		return nil, &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		lErr := tx.Rollback()
		if err == nil && lErr != nil {
			err = &Error{Err: err, Description: "Couldn't end transaction"}
		}
	}()

	competitionBucket := tx.Bucket([]byte("competition"))
	if competitionBucket == nil {
		return nil, nil
	}

	return readCompetition(competitionBucket)
}

func (db *boltDB) writeRevision(tx *bolt.Tx) (err error) {
	//read old competition
	competitionBucket := tx.Bucket([]byte("competition"))
	if competitionBucket == nil {
		return nil
	}

	old, err := readCompetition(competitionBucket)
	if err != nil {
		return &Error{Err: err, Description: "Couldn't read competition"}
	}

	//read old last modified
	configBucket := competitionBucket.Bucket([]byte("config"))
	if configBucket == nil {
		return &Error{Err: nil, Description: fmt.Sprintf("Competition(%s) config Bucket was nil", old.Name)}
	}

	lastModified := configBucket.Get([]byte("last_modified"))

	//create revision bucket
	revisionsBucket, err := tx.CreateBucketIfNotExists([]byte("revisions"))
	if err != nil {
		return &Error{Err: err, Description: "Couldn't create Database revisions Bucket"}
	}

	last, err := db.getLatestRevision(tx)
	if err != nil {
		return &Error{Err: err, Description: "Couldn't get latest Revision"}
	}

	revisionBucket, err := revisionsBucket.CreateBucketIfNotExists(intToBytes(last + 1))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Revision(%d) bucket", last)}
	}

	//write config
	configBucket, err = revisionBucket.CreateBucket([]byte("config"))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Revision(%d) config bucket", last)}
	}

	err = configBucket.Put([]byte("last_modified"), lastModified)
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Revision(%d) config.last_modified(%#v)", last, lastModified)}
	}

	//write competition
	competitionBucket, err = revisionBucket.CreateBucket([]byte("competition"))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Revision(%d) competition bucket", last)}
	}

	err = writeCompetition(competitionBucket, old)
	if err != nil {
		return &Error{Err: err, Description: "Couldn't write Revision"}
	}

	//write current revision
	configBucket = tx.Bucket([]byte("config"))
	if configBucket == nil {
		return &Error{Err: nil, Description: "Database config Bucket was nil"}
	}

	err = configBucket.Put([]byte("current_revision"), intToBytes(last+1))
	if err != nil {
		return &Error{Err: err, Description: "Couldn't write Database config.current_revision"}
	}

	return nil
}

func (db *boltDB) Write(c *Competition) (err error) {
	tx, err := db.Begin(true)
	if err != nil {
		return &Error{Err: err, Description: "Couldn't start transaction"}
	}
	defer func() {
		if err != nil {
			lErr := tx.Rollback()
			if lErr != nil {
				err = &Error{Err: lErr, Description: fmt.Sprintf("Couldn't rollback transaction; error causing rollback: %s", err)}
			}
			return
		}
		lErr := tx.Commit()
		if lErr != nil {
			err = &Error{Err: err, Description: "Couldn't commit transaction"}
		}
	}()

	//store current competition as a revision
	if competitionBucket := tx.Bucket([]byte("competition")); competitionBucket != nil {

		err = db.writeRevision(tx)
		if err != nil {
			return &Error{Err: err, Description: "Couldn't write Revision"}
		}

		//clear competition
		err = tx.DeleteBucket([]byte("competition"))
		if err != nil {
			return &Error{Err: err, Description: "Couldn't clear competition Bucket"}
		}
	}

	t, err := time.Now().MarshalBinary()
	if err != nil {
		return &Error{Err: err, Description: "Couldn't encode time"}
	}

	competitionBucket, err := tx.CreateBucket([]byte("competition"))
	if err != nil {
		return &Error{Err: err, Description: "Couldn't create competition Bucket"}
	}

	configBucket, err := competitionBucket.CreateBucket([]byte("config"))
	if err != nil {
		return &Error{Err: err, Description: "Couldn't create Competition config Bucket"}
	}

	if err = configBucket.Put([]byte("last_modified"), t); err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Competition config.last_modified(%v)", t)}
	}

	return writeCompetition(competitionBucket, c)
}
