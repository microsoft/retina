package file

import (
	"fmt"
	"strings"
)

type CaptureFilename struct {
	CaptureName    string
	NodeHostname   string
	StartTimestamp *Timestamp
}

func (cf *CaptureFilename) GenerateCaptureFileName() string {
	formattedTime := strings.Replace(cf.StartTimestamp.TimestampToString(), "#", "", -1)
	uniqueName := fmt.Sprintf("%s-%s-%s", cf.CaptureName, cf.NodeHostname, formattedTime)
	return uniqueName
}
