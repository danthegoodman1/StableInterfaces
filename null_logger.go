package stableinterfaces

import (
	"log"
	"os"
)

type (
	// NullLogger does not log ever, except for on Fatal
	NullLogger struct{}
)

func (dl *NullLogger) Debug(_ string)          {}
func (dl *NullLogger) Info(_ string)           {}
func (dl *NullLogger) Warn(_ string, _ error)  {}
func (dl *NullLogger) Error(_ string, _ error) {}
func (dl *NullLogger) Fatal(msg string, err error) {
	log.Println("StableInterface FTL:", msg, "--", err)
	os.Exit(1)
}
