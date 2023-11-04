package stableinterfaces

// Logger is used to enable logging
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(err error, msg string)
}
