package file

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CaptureFilename struct {
	CaptureName    string
	NodeHostname   string
	StartTimestamp *metav1.Time
}

func (cf *CaptureFilename) String() string {
	uniqueName := fmt.Sprintf("%s-%s-%s", cf.CaptureName, cf.NodeHostname, TimeToString(cf.StartTimestamp))
	return uniqueName
}
