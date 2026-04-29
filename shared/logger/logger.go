package logger

import (
	"time"

	"github.com/gocql/gocql"
)

type LogEntry struct {
	OriginService string
	ActionType    string
	Description   string
	OriginIP      string
	ResultStatus  string
	UserID        *gocql.UUID
}

type Logger struct {
	session *gocql.Session
}

func NewLogger(session *gocql.Session) *Logger {
	return &Logger{session: session}
}

func (l *Logger) Log(entry LogEntry) error {
	if l == nil || l.session == nil {
		return nil
	}

	logID := gocql.TimeUUID()
	now := time.Now()

	const query = `INSERT INTO register_logs
		(log_id, event_hour, reference_date, origin_service, action_type, description, origin_ip, result_status, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	return l.session.Query(query,
		logID,
		now,
		now,
		entry.OriginService,
		entry.ActionType,
		entry.Description,
		entry.OriginIP,
		entry.ResultStatus,
		entry.UserID,
	).Exec()
}
