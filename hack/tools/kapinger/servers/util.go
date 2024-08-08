package servers

import (
	"fmt"
	"os"
)

func getResponse(addressString, protocol string) []byte {
	podname := os.Getenv("POD_NAME")
	return []byte(fmt.Sprintf("connected to: %s via %s, connected from: %v", podname, protocol, addressString))
}
