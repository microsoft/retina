package file

import (
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const captureFileNameTimestampFormat string = "2006-01-02 15:04:05 +0000 MST"

func Now() *metav1.Time {
	return &metav1.Time{Time: time.Now().UTC().Truncate(time.Second)}
}

// Converts a string in the capture file name format to metav1.Time
func StringToTime(timestamp string) (*metav1.Time, error) {
	parsedTime, err := time.Parse(captureFileNameTimestampFormat, timestamp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create timestamp from string")
	}
	return &metav1.Time{Time: parsedTime}, nil
}

// Converts a metav1.Time to a string in the capture file name format
func TimeToString(timestamp *metav1.Time) string {
	return timestamp.Format(captureFileNameTimestampFormat)
}
