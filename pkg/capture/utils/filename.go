package utils

import (
	"fmt"
	"strings"
	"time"
)

const captureFileNameTimestampFormat string = "2006#01#02#15#04#05UTC"

func GenerateCaptureFileName(captureName, nodeHostname string, startTimestampUTC time.Time) string {
	formattedUTCTime := strings.Replace(ConvertTimestampToString(startTimestampUTC), "#", "", -1)
	uniqueName := fmt.Sprintf("%s-%s-%s", captureName, nodeHostname, formattedUTCTime)
	return uniqueName
}

func ConvertTimestampToString(timestamp time.Time) string {
	return timestamp.Format(captureFileNameTimestampFormat)
}

func ConvertStringToTimestamp(timestamp string) (time.Time, error) {
	timestampStr, err := time.Parse(captureFileNameTimestampFormat, timestamp)
	if err != nil {
		return time.Time{}, err
	}
	return timestampStr, nil
}
