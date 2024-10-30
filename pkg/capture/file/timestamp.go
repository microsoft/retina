package file

import (
	"time"

	"github.com/pkg/errors"
)

type Timestamp struct {
	time.Time
}

const captureFileNameTimestampFormat string = "2006#01#02#15#04#05UTC"

func (timestamp *Timestamp) TimestampToString() string {
	return timestamp.Format(captureFileNameTimestampFormat)
}

func NewTimestamp(timestamp string) (*Timestamp, error) {
	timestampStr, err := time.Parse(captureFileNameTimestampFormat, timestamp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new timestamp")
	}
	return &Timestamp{timestampStr}, nil
}
