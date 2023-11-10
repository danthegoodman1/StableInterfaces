package stableinterfaces

// Logger is used to enable logging
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string, err error)
	Error(msg string, err error)
	// Fatal should log, then exit the process with os.Exit(1)
	Fatal(msg string, err error)
}
