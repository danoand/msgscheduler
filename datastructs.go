package main

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github/danoand/utils"

	"github.com/boltdb/bolt"
	"labix.org/v2/mgo/bson"
)

const maxMsgLen = 1599

var rgxNonDigit = regexp.MustCompile("\\D")

// item represents a message to be scheedule for future deliver
type item struct {
	ID     bson.ObjectId `json:"id"`     // unique identifier of this scheduled message
	Active bool          `json:"active"` // indicator if the message is active (true) or not (false)
	Msg    string        `json:"msg"`    // body of the text message to be sent
	Number string        `json:"number"` // ten digit phone number to which the message is to be sent
	Time   time.Time     `json:"time"`   // earliest time for the message to be sent
}

// newItem creates a new item object
func newItem(num, msg string, tme time.Time) (item, error) {
	var (
		retItem  item
		tmpPhone string
		ferr     error
	)

	// Validate the inbound parameters
	tmpPhone = rgxNonDigit.ReplaceAllString(num, "")
	if len(tmpPhone) != 10 {
		// Error - invalid phone number (not 10 numeric digits)
		return retItem, fmt.Errorf("invalid phone number - not 10 numeric digits")
	}

	if len(msg) == 0 || len(msg) > 1599 {
		// Invalid message parameter
		return retItem, fmt.Errorf("invalid message length - must be > 0 and < 1600 characters")
	}

	if tme.Before(time.Now()) {
		// Error - the requested time is now in the past
		return retItem, fmt.Errorf("the scheduled message time is in the past")
	}

	// Construct the item
	retItem.ID = bson.NewObjectId()
	retItem.Active = true
	retItem.Msg = msg
	retItem.Number = tmpPhone
	retItem.Time = tme

	return retItem, ferr
}

// isActive is a method on item that indicates the if the item is active
func (itm item) isActive() bool {
	return itm.Active
}

// deActivate is a method on item that marks an item as inactive
func (itm *item) deActivate() {
	itm.Active = false
}

// activate is a method on item that marks an item as active
func (itm *item) activate() {
	itm.Active = true
}

// save is a method on item that saves the item to the database
func (itm item) save() error {
	var ferr error
	var jbytes []byte

	// Convert the item to json
	_, jbytes, ferr = utils.ToJSON(itm)
	if ferr != nil {
		// Error occurred marshaling an item to json
		log.Printf("ERROR: error occurred converting an message item to JSON. See: %v\n", ferr)
		return ferr
	}

	// Save the item to the database
	ferr = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("ScheduledMsgs"))
		berr := b.Put([]byte(itm.ID.Hex()), jbytes)
		return berr
	})
	if ferr != nil {
		// Error occurred marshaling an item to json
		log.Printf("ERROR: error occurred saving a message schedule item to the database. See: %v\n", ferr)
	}

	return ferr
}

// execute is a method on item that sends the scheduled item's message
func (itm item) execute() {
	var ferr error

	// Call to Twilio to send the scheduled text message
	if !sendTwilioText(itm.Number, itm.Msg) {
		// Error - call to Twilio failed
		log.Printf("ERROR: Twilio call to send scheduled message %v failed\n", itm.ID)
		return
	}

	log.Printf("INFO: Scheduled message %v has been sent\n", itm.ID)

	// Deactivate the item
	itm.deActivate()

	// Save the item back to the database
	ferr = itm.save()
	if ferr != nil {
		log.Printf("ERROR: error occurred saving the item back to the database. See: %v\n", itm.ID)
	}
	return
}
