// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package client

import (
	"fmt"
	"net/http"
	"time"
)

// Retina API
const (
	startTrace = "%s/trace"
	trace      = "%s/trace/%s"
)

type Retina struct {
	RetinaEndpoint string
	Client         *http.Client
}

func NewRetinaClient(endpoint string) *Retina {
	return &Retina{
		RetinaEndpoint: endpoint,
		Client:         &http.Client{Timeout: time.Second * 30},
	}
}

func (c *Retina) StartTrace(operationID, filter string) error {
	startTraceURL := fmt.Sprintf(startTrace, c.RetinaEndpoint)
	response, err := c.Client.Post(startTraceURL, "application/json", nil)
	if err != nil {
		return err
	}

	response.Body.Close()
	return nil
}

func (c *Retina) GetTrace(operationID string) error {
	getTraceURL := fmt.Sprintf(trace, c.RetinaEndpoint, operationID)
	response, err := c.Client.Get(getTraceURL)
	if err != nil {
		return err
	}

	response.Body.Close()
	return nil
}
