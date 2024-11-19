package file

import (
	"fmt"
)

type CaptureFilename struct {
	CaptureName    string
	NodeHostname   string
	StartTimestamp *Timestamp
}

func (cf *CaptureFilename) String() string {
	uniqueName := fmt.Sprintf("%s-%s-%s", cf.CaptureName, cf.NodeHostname, cf.StartTimestamp)
	return uniqueName
}
