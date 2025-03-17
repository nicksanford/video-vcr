package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"go.viam.com/utils"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
	_ "github.com/mattn/go-sqlite3"
)

// 2025/03/13 16:53:00 conn opened
// 2025/03/13 16:53:00 describe request
// 2025/03/13 16:53:00 session opened
// 2025/03/13 16:53:00 setup request
// 2025/03/13 16:53:00 play request
// 2025/03/13 16:53:01 conn closed (EOF)
// This example shows how to
// 1. create a RTSP server which accepts plain connections
// 2. allow a single client to publish a stream with TCP or UDP
// 3. allow multiple clients to read that stream with TCP, UDP or UDP-multicast

type serverHandler struct {
	s *gortsplib.Server
	// stream  *gortsplib.ServerStream
	db   *sql.DB
	desc description.Session

	connsMu sync.Mutex
	conns   map[uuid.UUID]*conn
	// workers map[*gortsplib.ServerSession]*utils.StoppableWorkers
	// starts  map[*gortsplib.ServerSession]context.CancelFunc
	// streams map[*gortsplib.ServerSession]*gortsplib.ServerStream

	// publisher *gortsplib.ServerSession
	// desc description.Session
}

func (sh *serverHandler) close() {
	sh.connsMu.Lock()
	defer sh.connsMu.Unlock()
	for _, conn := range sh.conns {
		conn.close()
	}
}

type conn struct {
	uuid    uuid.UUID
	created time.Time
	rconn   *gortsplib.ServerConn
	rserver *gortsplib.Server
	worker  *utils.StoppableWorkers
	mu      sync.Mutex
	stream  *gortsplib.ServerStream
}

func (c *conn) close() {
	log.Printf("conn close start")
	defer log.Printf("conn close end")
	c.mu.Lock()
	stream := c.stream
	c.stream = nil
	c.mu.Unlock()

	if stream != nil {
		stream.Close()
	}
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
	conn := &conn{
		uuid:    uuid.New(),
		created: time.Now(),
		rconn:   ctx.Conn,
		rserver: sh.s,
		stream:  gortsplib.NewServerStream(sh.s, &sh.desc),
		worker:  utils.NewBackgroundStoppableWorkers(),
	}

	// todo: figure out when to delete these
	sh.connsMu.Lock()
	sh.conns[conn.uuid] = conn
	sh.connsMu.Unlock()
	ctx.Conn.SetUserData(conn)
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	stats := ctx.Conn.Stats()
	conn := ctx.Conn.UserData().(*conn)
	conn.close()
	conn.worker.Stop()
	log.Printf("conn closed (%v), BytesSent: %d, BytesReceived: %d", ctx.Error, stats.BytesSent, stats.BytesReceived)
}

// called when receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("describe request")
	conn := ctx.Conn.UserData().(*conn)
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.stream == nil {
		return &base.Response{StatusCode: base.StatusBadRequest}, nil, nil
	}

	// send medias that are being published to the client
	return &base.Response{
		StatusCode: base.StatusOK,
	}, conn.stream, nil
}

// called when receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(_ *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	log.Printf("announce request")
	return &base.Response{
		StatusCode: base.StatusBadRequest,
	}, nil
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("onsetup")
	conn := ctx.Conn.UserData().(*conn)
	conn.mu.Lock()
	stream := conn.stream
	conn.mu.Unlock()
	if stream == nil {
		return &base.Response{StatusCode: base.StatusBadRequest}, nil, nil
	}
	return &base.Response{StatusCode: base.StatusOK}, stream, nil
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	stats := ctx.Session.Stats()
	log.Printf("session closed (%v), BytesSent: %d, BytesReceived: %d", ctx.Error, stats.BytesSent, stats.BytesReceived)
}

const buffer = time.Millisecond * 200

// called when receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("OnPlay START")
	defer log.Printf("OnPlay END")
	conn := ctx.Conn.UserData().(*conn)
	conn.mu.Lock()
	defer conn.mu.Unlock()
	stream := conn.stream
	if stream == nil {
		log.Printf("OnPlay stream is nil")
		return &base.Response{StatusCode: base.StatusBadRequest}, nil
	}

	// if conn.worker != nil {
	// 	return &base.Response{
	// 		StatusCode: base.StatusOK,
	// 	}, nil
	// }

	fmt.Println("spawning worker")
	conn.worker.Add(
		func(stopCtx context.Context) {
			fmt.Println("worker started")
			defer conn.close()
			if stopCtx.Err() != nil {
				return
			}

			fmt.Print("running query")
			rows, err := sh.db.Query("select id, data, timeDeltaMicro from video where id > 1;")
			if err != nil {
				panic(err)
			}
			fmt.Print("running query done")
			var (
				id             int
				data           []byte
				timeDeltaMicro int64
				startTime      time.Time
				now            time.Time
			)

			for {
				if stopCtx.Err() != nil {
					return
				}

				if !rows.Next() {
					log.Println("no more rows")
					return
				}

				if err := rows.Scan(&id, &data, &timeDeltaMicro); err != nil {
					panic(err)
				}

				pkt := &rtp.Packet{}
				if err := pkt.Unmarshal(data); err != nil {
					panic(err)
				}

				if startTime.IsZero() {
					startTime = time.Now()
				}

				packetTime := startTime.Add(time.Duration(timeDeltaMicro) * time.Microsecond)
				for {
					now = time.Now()
					if packetTime.Before(now.Add(buffer)) {
						// fmt.Printf("sending packet %d, deltaMS: %d\n", id-1, (time.Duration(timeDeltaMicro) * time.Microsecond).Milliseconds())
						err := stream.WritePacketRTP(sh.desc.Medias[0], pkt)
						if err != nil {
							return
						}
						break
					}
					time.Sleep(time.Microsecond * 50)
				}

				// if the packet is before 100 ms in the future, send it and continue
				// otherwise, wait 50 ms and try again

			}
		})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called when receiving a RECORD request.
func (sh *serverHandler) OnRecord(_ *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	log.Printf("record request")

	// called when receiving a RTP packet
	return &base.Response{
		StatusCode: base.StatusBadRequest,
	}, nil
}

func Run(dbPath string) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(err)
	}
	rows, err := db.Query("select id, data from video where id = 1;")
	if err != nil {
		panic(err)
	}
	// TODO: Shove r on the ;serverHandler & read from it for each packet
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Printf("err on db close: %s", err.Error())
		}
	}()
	if !rows.Next() {
		panic("no first row")
	}

	var id int
	var data []byte
	if err := rows.Scan(&id, &data); err != nil {
		panic(err)
	}

	if err := rows.Close(); err != nil {
		panic(err)
	}

	var sdp sdp.SessionDescription
	if err := sdp.Unmarshal(data); err != nil {
		panic(err)
	}

	var desc description.Session
	if err := desc.Unmarshal(&sdp); err != nil {
		panic(err)
	}

	var videoMedias []*description.Media
	for _, m := range desc.Medias {
		if m.Type == description.MediaTypeVideo {
			videoMedias = append(videoMedias, m)
		}
	}
	if len(videoMedias) != 1 {
		panic("expected only one video stream")
	}

	log.Printf("desc:	%#v", desc)
	for _, m := range desc.Medias {
		log.Printf("medi:	%#v", m)
		for _, f := range m.Formats {
			log.Printf("format:	%#v", f)
		}
	}

	h := &serverHandler{
		db:    db,
		desc:  desc,
		conns: map[uuid.UUID]*conn{},
	}
	h.s = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}

	// start server and wait until a fatal error
	log.Printf("server is ready")
	panic(h.s.StartAndWait())
}
