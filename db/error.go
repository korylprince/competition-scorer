package db

import (
	"fmt"
)

//Error represents a DB error
type Error struct {
	Err         error
	Description string
}

//Error fufills the error interface
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Description, e.Err.Error())
	}
	return e.Description
}
