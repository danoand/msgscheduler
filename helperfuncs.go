package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/danoand/utils"
	"labix.org/v2/mgo/bson"
)

const (
	twilioAccountSID = "AC1661d55ca882016d8e4c038c47acc8bc"
	twilioAuthToken  = "62d5f0c0455961f085641e58963fc1d0"
	twilioNumber     = "+16302257108"
)

var (
	// Create a regular expression to match "yyyy-mm-dd hh:mm" or "yyyy-mm-dd hh:mm"
	//   Assume a "24 hour clock"
	dateTime = regexp.MustCompile("\\d\\d\\d\\d-\\d\\d-\\d\\d \\d\\d:\\d\\d")

	// Set up the elements of the REST API URL
	twilioURLElement1         = "https://"
	twilioURLElement2         = ":"
	twilioURLElement3         = "@api.twilio.com/2010-04-01/Accounts/"
	twilioURLElement4         = "/Messages.json"
	twilioConstructMessageURL = twilioURLElement1 + twilioAccountSID + twilioURLElement2 + twilioAuthToken +
		twilioURLElement3 + twilioAccountSID + twilioURLElement4
)

// toNums generates a set of integers from a valid date/time value
func toNums(dt string) (y, m, d, h, m int, ferr error) {
	var errMsg = "error: %v value can't be converted to an integer; see: %v"
	var iter = []string{"year", "month", "day", "hour", "minute"}

	// Iterate through the integer data value types
	for _, v := range iter {
		switch v {

		case "year":
			y, ferr = strconv.Atoi(s)
			if ferr != nil {
				return y, m, d, h, m, fmt.Errorf(errMsg, v, ferr)
			}

		case "month":
			m, ferr = strconv.Atoi(s)
			if ferr != nil {
				return y, m, d, h, m, fmt.Errorf(errMsg, v, ferr)
			}

		case "day":
			d, ferr = strconv.Atoi(s)
			if ferr != nil {
				return y, m, d, h, m, fmt.Errorf(errMsg, v, ferr)
			}

		case "hour":
			h, ferr = strconv.Atoi(s)
			if ferr != nil {
				return y, m, d, h, m, fmt.Errorf(errMsg, v, ferr)
			}

		case "minute":
			m, ferr = strconv.Atoi(s)
			if ferr != nil {
				return y, m, d, h, m, fmt.Errorf(errMsg, v, ferr)
			}
		}
	}

	// Return to the caller
	return y, m, d, h, m, ferr
}

// toTime converts a string representing a date/time to a Go time object
func toTime(s string) (time.Time, error) {
	var ferr error
	var y, m, d, h, m int
	var retTime time.Time

	// Validate the date/time string
	if !dateTime.MatchString(s) {
		// Date/time string is invalid
		return retTime, fmt.Errorf("invalid datetime value")
	}

	// Convert the date/time value to a set of integer values
	y, m, d, h, m, ferr = toNums(s)
	if ferr != nil {
		// Error occurred translating the date/time elements to integers
		return retTime, ferr
	}

	// Create a time object using the date/time data
	retTime = time.Date(y, m, d, h, m, 0, 0, timeLoc)

	return retTime, ferr
}

// toItems converts a slice of json data (slice of bytes) represting scheduled items into a slice Go item objects
func toItems(jsn [][]byte) []item {
	var ferr error
	var tmpMap map[string]interface{}
	var tmpItem item
	var retItems []item

	// Iterate through the slice of byte slices
	for i := 0; i < len(jsn); i++ {
		// Skip if json is "empty"
		if len(jsn[i]) == 0 {
			continue
		}

		ferr = utils.FromJSONBytes(jsn[i], &tmpMap)
		if ferr != nil {
			log.Printf("ERROR: error occurred parsing a json byte slice. See: %v\n", ferr)
			continue
		}

		// Construct an item
		tmpItem.Active = jsn[i]["active"]
		if !bson.IsObjectIdHex(jsn[i]["id"]) {
			// Error - the id value is not a valid bson objectid - skip
			log.Printf("ERROR: schedule id is not a valid bson objectid\n")
			continue
		}

		tmpItem.ID = bson.ObjectIdHex(jsn[i]["id"])
		tmpItem.Msg = jsn[i]["msg"]

		tmpItem.Time, ferr = toTime(jsn[i]["time"])
		if ferr != nil {
			// Error occurred translating the date/time stamp to a Go time object
			log.Printf("ERROR: error occurred translating the date/time stamp to a Go time object. See: %v\n", ferr)
			continue
		}

		// Add scheduled item
		retItems = append(retItems, tmpItem)
	}

	return retItems
}

// sendTwilioText triggers a Text message via Twilio
func sendTwilioText(phn, msg string) bool {
	var (
		rErr   error
		retVal = true
		max    int
		twResp *http.Response
	)

	// Set the maxlength of the body to be sent
	if max = len(msg); max > 1600 {
		max = 1600
	}

	// Truncate the message text to a max length (if needed)
	msg = msg[0:max]

	// Construct the parameters to be sent via the HTTP POST
	urlVals := url.Values{}
	urlVals.Set("From", twilioNumber)
	urlVals.Set("To", phn)
	urlVals.Set("Body", msg)

	// Configure an HTTP POST
	if twResp, rErr = http.PostForm(twilioConstructMessageURL, urlVals); rErr != nil {
		log.Printf("ERROR: error posting text message to Twilio. See: %v\n", rErr)
		retVal = false
	}

	// Log the Twilo response
	log.Printf("INFO: Scheduled text message fired. Status of Twilio response: %v\n", twResp.Status)

	// Return to caller
	return retVal
}

// deleteItems deletes deactive items from the database
func deleteItems() {
	var ferr error
	var items []item

	// Fetch scheduled items from the database
	items, ferr = fetchSchd()
	if ferr != nil {
		// Error occurred fetching scheduled items from the database
		log.Printf("ERROR: Error occurred fetching scheduled items from the database. See: %v\n", ferr)
	}

	// Iterate over the items
	for i := 0; i < len(items); i++ {
		// Is this current item inactive?
		if !items[i].isActive() {
			// Item is inactive. Delete the associated key-value pair in the database
			ferr = db.Update(func(tx *bolt.Tx) error {
				return tx.Bucket([]byte("ScheduledMsgs")).Delete([]byte(items[i].ID.Hex()))
			})

			if ferr != nil {
				log.Printf("ERROR: error occurred deleting key: %v. See: %v\n", items[i].ID.Hex(), ferr)
			}
		}
	}
}
