package file

import (
	"time"

	"github.com/pkg/errors"
)

type Timestamp struct {
	time.Time
}

const captureFileNameTimestampFormat string = "20060102150405UTC"

func Now() Timestamp {
	return Timestamp{Time: time.Now().UTC().Truncate(time.Second)}
}

func (timestamp *Timestamp) String() string {
	return timestamp.Time.Format(captureFileNameTimestampFormat)
}

func StringToTimestamp(timestamp string) (*Timestamp, error) {
	parsedTime, err := time.Parse(captureFileNameTimestampFormat, timestamp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create timestamp from string")
	}
	return &Timestamp{parsedTime}, nil
}

// TimeToString converts a Timestamp to a string in the capture file name format.
func TimeToString(t Timestamp) string {
	return t.Time.UTC().Format(captureFileNameTimestampFormat)
}
