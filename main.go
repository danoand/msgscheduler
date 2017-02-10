package main

import (
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
)

var timeLoc *time.Location

// schedMsg schedules a future text message into the database
func schedMsg(msg, schd string) error {
	var ferr error

	return ferr
}

// fetchSchd fetches outstanding messages marked as active (to be sent)
func fetchSchd() ([]item, error) {
	var ferr error
	var retItems []item
	var vbytes [][]byte

	// Iterate over the the database key-value pairs
	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("ScheduledMsgs"))

		c := b.Cursor()

		// Loop over each key value pair
		for _, v := c.First(); kbytes != nil; _, v = c.Next() {
			vbytes = append(vbytes, v)
		}

		return nil
	})

	// Are there any key values to process?
	if len(vbytes) == 0 {
		// Nothing returned from the database query - return
		return retItems, ferr
	}

	// Convert the key values to Go item objects
	retItems, ferr = toItems(vbytes)

	return retItems, ferr
}

// procSchd processes a set of scheduled items
func procSchd(itms []item) {
	var now = time.Now()

	// Iterate through the slice of scheduled items
	for i := 0; i < len(itms); i++ {
		// Has the item been scheduled for a time in the past (before now-ish)
		if itms[i].Time.Before(now) {
			// Execute this schedule item
			go itms[i].execute()
		}
	}
}

// schedJobs defines the cron jobs (called when starting the application up)
func schedJobs() {
	log.Printf("INFO: define cron jobs\n")

	// Define recurring jobs
	gocron.Every(1).Day().At("01:30").Do(deleteItems()) // Delete inactive scheduled items

	// Start the default gocron scheduler
	log.Printf("INFO: start 'cron' scheduler\n")
	<-gocron.Start()

	log.Printf("INFO: stop cron jobs\n")

	return
}

func init() {
	var ierr error

	// Load the app's processing time zone
	timeLoc, ierr = time.LoadLocation("America/Chicago")
	if ierr != nil {
		log.Fatalf("FATAL: error occurred loading the app's timezone. See: %v\n", ierr)
	}
}

func main() {
	// Open the application database
	db, dberr := bolt.Open("app.db", 0600, nil)
	if dberr != nil {
		// Error opening the local database
		log.Fatalf("FATAL: error opening the local database. See: %v\n", dberr)
	}

	// Remember to close the database
	defer db.Close()

	// Ensure expected database bucket(s) exist
	bkerr = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("ScheduledMsgs"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if bkerr != nil {
		// Error referencing a required database bucket
		log.Fatalf("error creating a required database bucket")
	}

	// Schedule recurring "cron" jobs
	go schedJobs()

}
