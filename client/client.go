package client

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"

	_ "github.com/mattn/go-sqlite3"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. read all media streams on a path.
func Run(url, dbPath string) {
	c := gortsplib.Client{}
	if _, err := os.Stat(dbPath); err == nil {
		os.Remove(dbPath)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = db.Close(); err != nil {
			panic(err)
		}
	}()

	sqlStmt := `
    CREATE TABLE video(id INTEGER NOT NULL PRIMARY KEY, data BLOB, timeDeltaMicro INTEGER);
	`

	if _, err = db.Exec(sqlStmt); err != nil {
		panic(err)
	}

	// parse URL
	u, err := base.ParseURL(url)
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find available medias
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}
	log.Printf("desc: %#v\n", desc)
	descData, err := desc.Marshal(false)
	if err != nil {
		panic(err)
	}

	// setup all medias
	err = c.SetupAll(desc.BaseURL, desc.Medias)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	var (
		t         time.Time
		startTime time.Time
	)
	c.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		if medi.Type != description.MediaTypeVideo {
			return
		}
		pktData, cbErr := pkt.Marshal()
		if cbErr != nil {
			log.Printf("ERROR RTP packet from media %#v, format: %#v, err: %s\n", medi, forma, cbErr.Error())
			return
		}

		t = time.Now()
		if startTime.IsZero() {
			startTime = t
		}

		_, cbErr = db.Exec("INSERT INTO video(data, timeDeltaMicro) VALUES(?, ?);", pktData, t.Sub(startTime).Microseconds())
		if err != nil {
			panic(cbErr)
		}
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}
	if _, err = db.Exec("INSERT INTO video(data) VALUES(?);", descData); err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
