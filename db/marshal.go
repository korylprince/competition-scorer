package db

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/boltdb/bolt"
)

func bytesToInt(data []byte) (int32, error) {
	b := bytes.NewBuffer(data)
	var i int32
	err := binary.Read(b, binary.BigEndian, &i)
	return i, err
}

func intToBytes(i int32) []byte {
	b := new(bytes.Buffer)
	err := binary.Write(b, binary.BigEndian, i)
	if err != nil {
		panic(fmt.Errorf("Couldn't convert int to []byte: %v", err))
	}
	return b.Bytes()
}

func readTeam(b *bolt.Bucket, rounds int) (*Team, error) {
	t := &Team{
		Name:   string(b.Get([]byte("name"))),
		Scores: make([]*int32, rounds),
	}
	if t.Name == "" {
		return nil, &Error{Err: nil, Description: "Team name was empty"}
	}

	scoresBucket := b.Bucket([]byte("scores"))
	if scoresBucket == nil {
		return nil, &Error{Err: nil, Description: fmt.Sprintf("Team(%s) scores Bucket was nil", t.Name)}
	}

	for i := 0; i < rounds; i++ {
		if score := scoresBucket.Get(intToBytes(int32(i))); score != nil {
			val, err := bytesToInt(score)
			if err != nil {
				return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't decode Team(%s) Round(%d) score(%#v)", t.Name, i, score)}
			}
			t.Scores[i] = &val
		} else {
			t.Scores[i] = nil
		}
	}

	return t, nil
}

func writeTeam(b *bolt.Bucket, t *Team, rounds int) error {
	err := b.Put([]byte("name"), []byte(t.Name))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Team(%s) name", t.Name)}
	}

	if len(t.Scores) != rounds {
		return &Error{Err: nil, Description: fmt.Sprintf("Team(%s) Rounds(%d) doesn't match Competition Rounds(%d)", t.Name, len(t.Scores), rounds)}
	}

	scoresBucket, err := b.CreateBucketIfNotExists([]byte("scores"))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Team(%s) scores Bucket", t.Name)}
	}

	for i := 0; i < rounds; i++ {
		if t.Scores[i] == nil {
			continue
		}

		err = scoresBucket.Put(intToBytes(int32(i)), intToBytes(*t.Scores[i]))
		if err != nil {
			return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Team(%s) Round(%d) Score(%d)", t.Name, i, t.Scores[i])}
		}
	}

	return nil
}

func readCompetition(b *bolt.Bucket) (*Competition, error) {
	name := string(b.Get([]byte("name")))
	if name == "" {
		return nil, &Error{Err: nil, Description: "Competition name was empty"}
	}

	configBucket := b.Bucket([]byte("config"))
	if configBucket == nil {
		return nil, &Error{Err: nil, Description: fmt.Sprintf("Competition(%s) config Bucket was nil", name)}
	}

	roundsBytes := configBucket.Get([]byte("rounds"))
	rounds, err := bytesToInt(roundsBytes)
	if err != nil {
		return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't decode Competition(%s) config.rounds(%#v)", name, roundsBytes)}
	}

	teamsBytes := configBucket.Get([]byte("teams"))
	teams, err := bytesToInt(teamsBytes)
	if err != nil {
		return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't decode Competition(%s) config.teams(%#v)", name, teamsBytes)}
	}

	c := &Competition{
		Name:   name,
		Rounds: make([]string, rounds),
		Teams:  make([]*Team, teams),
	}

	roundsBucket := b.Bucket([]byte("rounds"))
	if roundsBucket == nil {
		return nil, &Error{Err: nil, Description: fmt.Sprintf("Competition(%s) rounds Bucket was nil", name)}
	}

	for i := 0; i < int(rounds); i++ {
		c.Rounds[i] = string(roundsBucket.Get(intToBytes(int32(i))))
		if c.Rounds[i] == "" {
			return nil, &Error{Err: nil, Description: fmt.Sprintf("Competition(%s) Round(%d) was empty", name, i)}
		}
	}

	teamsBucket := b.Bucket([]byte("teams"))
	if teamsBucket == nil {
		return nil, &Error{Err: nil, Description: fmt.Sprintf("Competition(%s) teams Bucket was nil", name)}
	}

	for i := 0; i < int(teams); i++ {
		team, err := readTeam(teamsBucket.Bucket(intToBytes(int32(i))), int(rounds))
		if err != nil {
			return nil, &Error{Err: err, Description: fmt.Sprintf("Couldn't read Competition(%s) Team (%d)", name, i)}
		}

		c.Teams[i] = team
	}

	return c, nil
}

func writeCompetition(b *bolt.Bucket, c *Competition) error {
	err := b.Put([]byte("name"), []byte(c.Name))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Competition(%s) name", c.Name)}
	}

	configBucket, err := b.CreateBucketIfNotExists([]byte("config"))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Competition(%s) config Bucket", c.Name)}
	}

	err = configBucket.Put([]byte("rounds"), intToBytes(int32(len(c.Rounds))))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Competition(%s) config.rounds(%d)", c.Name, len(c.Rounds))}
	}

	err = configBucket.Put([]byte("teams"), intToBytes(int32(len(c.Teams))))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Competition(%s) config.teams(%d)", c.Name, len(c.Teams))}
	}

	roundsBucket, err := b.CreateBucketIfNotExists([]byte("rounds"))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Competition(%s) rounds Bucket", c.Name)}
	}

	for i := 0; i < len(c.Rounds); i++ {
		err = roundsBucket.Put(intToBytes(int32(i)), []byte(c.Rounds[i]))
		if err != nil {
			return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Competition(%s) Round(%d) name(%s)", c.Name, i, c.Rounds[i])}
		}
	}

	teamsBucket, err := b.CreateBucketIfNotExists([]byte("teams"))
	if err != nil {
		return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Competition(%s) teams Bucket", c.Name)}
	}

	for i := 0; i < len(c.Teams); i++ {
		teamBucket, err := teamsBucket.CreateBucketIfNotExists(intToBytes(int32(i)))
		if err != nil {
			return &Error{Err: err, Description: fmt.Sprintf("Couldn't create Competition(%s) Team(%d) Bucket", c.Name, i)}
		}

		err = writeTeam(teamBucket, c.Teams[i], len(c.Rounds))
		if err != nil {
			return &Error{Err: err, Description: fmt.Sprintf("Couldn't write Competition(%s) Time(%d)", c.Name, i)}
		}
	}

	return nil
}
