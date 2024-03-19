package generic

import (
	"flag"
	"fmt"
	"log"
	"os"
)

const DefaultTagEnv = "TAG"

type LoadTag struct {
	TagEnv string
}

func (s *LoadTag) Run() error {
	tag := os.Getenv(s.TagEnv)
	log.Printf("tag is %s\n", tag)
	return nil
}

func (s *LoadTag) Prevalidate() error {
	tag := os.Getenv(s.TagEnv)
	if tag != "" {
		log.Printf("tag is %s", tag)
	} else {
		log.Printf("tag is not set from env %s", s.TagEnv)

		var tag string
		flag.StringVar(&tag, "tag", "", "the tag to use for tests, like docker image tag")
		if tag != "" {
			log.Printf("using version \"%s\" from flag", tag)
			os.Setenv(s.TagEnv, tag)
			return nil
		} else {
			return fmt.Errorf("tag is not set from flag nor env %s", s.TagEnv)
		}
	}
	return nil
}

func (s *LoadTag) Postvalidate() error {
	return nil
}

func (s *LoadTag) Stop() error {
	return nil
}
