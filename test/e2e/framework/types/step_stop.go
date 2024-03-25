package types

import (
	"fmt"
	"log"
	"reflect"
)

type Stop struct {
	BackgroundID string
	Step         Step
}

func (c *Stop) Run() error {
	stepName := reflect.TypeOf(c.Step).Elem().Name()
	log.Println("stopping step:", stepName)
	err := c.Step.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop step: %s with err %w", stepName, err)
	}
	return nil
}

func (c *Stop) Stop() error {
	return nil
}

func (c *Stop) Prevalidate() error {
	return nil
}
