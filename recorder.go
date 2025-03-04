package vcr

import (
	"errors"
	"os"
	"path"
	"sync"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

type Recorder struct {
	logger logging.Logger
	dbPath string
	mu     sync.Mutex
	db     *sql.DB
}

func NewRecorder(dbPath string, logger logging.Logger) (*Recorder, error) {
	return &Recorder{dbPath: dbPath, logger: logger}, nil
}
func (rs *Recorder) InitH264(width, height int) error {
	return rs.init(true, width, height)
}

func (rs *Recorder) InitH265(width, height int) error {
	return rs.init(false, width, height)
}

func (rs *Recorder) init(isH264 bool, width, height int) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.db != nil {
		return errors.New("Init called multiple times")
	}

	if err := os.MkdirAll(path.Dir(rs.dbPath), 0o755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", rs.dbPath)
	if err != nil {
		return err
	}
	g := utils.NewGuard(func() {
		if cErr := db.Close(); cErr != nil {
			rs.logger.Error(cErr.Error())
		}
		if cErr := os.Remove(rs.dbPath); cErr != nil {
			rs.logger.Error(cErr.Error())
		}
	})

	sqlStmt := `
    CREATE TABLE extradata(id INTEGER NOT NULL PRIMARY KEY, isH264 BOOLEAN, width INTEGER, height INTEGER);
    CREATE TABLE packet(id INTEGER NOT NULL PRIMARY KEY, pts INTEGER,dts INTEGER,isIDR BOOLEAN, data BLOB);
	`

	if _, err := db.Exec(sqlStmt); err != nil {
		return err
	}

	if _, err = db.Exec("INSERT INTO extradata(isH264, width, height) VALUES(?, ?, ?);", isH264, width, height); err != nil {
		return err
	}
	g.Success()
	rs.db = db
	return nil
}

func (rs *Recorder) Packet(payload []byte, pts int64, dts int64, isIDR bool) error {
	if rs.db == nil {
		return errors.New("vcr.Recorder not initialized")
	}
	if _, err := rs.db.Exec("INSERT INTO packet(pts, dts, isIDR, data) VALUES(?, ?, ?, ?);", pts, dts, isIDR, payload); err != nil {
		return err
	}
	return nil
}

func (rs *Recorder) Close() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.db == nil {
		return nil
	}
	if err := rs.db.Close(); err != nil {
		return err
	}
	rs.db = nil
	return nil
}
