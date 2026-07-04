package gatewayadapter

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"
)

type Logger struct {
	out *log.Logger
}

func NewLogger(w io.Writer) *Logger {
	if w == nil {
		w = os.Stdout
	}
	return &Logger{out: log.New(w, "", 0)}
}

func (l *Logger) Info(event string, fields map[string]any) {
	l.write("info", event, fields)
}

func (l *Logger) Warn(event string, fields map[string]any) {
	l.write("warn", event, fields)
}

func (l *Logger) Error(event string, fields map[string]any) {
	l.write("error", event, fields)
}

func (l *Logger) Debug(enabled bool, event string, fields map[string]any) {
	if !enabled {
		return
	}
	l.write("debug", event, fields)
}

func (l *Logger) write(level string, event string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["level"] = level
	fields["event"] = event
	fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(fields)
	if err != nil {
		l.out.Printf(`{"level":"error","event":"logger_marshal_failed","error":%q}`, err.Error())
		return
	}
	l.out.Print(string(b))
}
