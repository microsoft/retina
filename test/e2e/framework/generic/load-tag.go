package generic

import (
	"flag"
	"fmt"
	"log"
	"os"
)

const (
	DefaultTagEnv         = "TAG"
	DefaultImageNamespace = "IMAGE_NAMESPACE"
	DefaultImageRegistry  = "IMAGE_REGISTRY"
	imageTagArg           = "image-tag"
	imageNamespaceArg     = "image-namespace"
	imageRegistryArg      = "image-registry"
)

var (
	ErrTagNotSet            = fmt.Errorf("tag not set")
	ErrImageNamespaceNotSet = fmt.Errorf("image namespace not set")
	ErrImageRegistryNotSet  = fmt.Errorf("image registry not set")
	imageTag                = flag.String(imageTagArg, "", "the image tag to use for tests")
	imageNamespace          = flag.String(imageNamespaceArg, "", "the image namespace to use for tests")
	imageRegistry           = flag.String(imageRegistryArg, "", "the image registry to use for tests")
)

type LoadFlags struct {
	TagEnv            string
	ImageNamespaceEnv string
	ImageRegistryEnv  string
}

func (s *LoadFlags) Run() error {
	tag := os.Getenv(s.TagEnv)
	imageNamespace := os.Getenv(s.ImageNamespaceEnv)
	imageRegistry := os.Getenv(s.ImageRegistryEnv)
	log.Printf("using image %s/%s:%s", imageRegistry, imageNamespace, tag)
	return nil
}

func (s *LoadFlags) Prevalidate() error {
	if err := s.validateEnvAndFlag(s.TagEnv, imageTagArg, *imageTag, ErrTagNotSet); err != nil {
		return err
	}

	if err := s.validateEnvAndFlag(s.ImageNamespaceEnv, imageNamespaceArg, *imageNamespace, ErrImageNamespaceNotSet); err != nil {
		return err
	}

	if err := s.validateEnvAndFlag(s.ImageRegistryEnv, imageRegistryArg, *imageRegistry, ErrImageRegistryNotSet); err != nil {
		return err
	}

	return nil
}

func (s *LoadFlags) validateEnvAndFlag(envVar, flagVar, flagValue string, err error) error {
	value := os.Getenv(envVar)
	if value != "" {
		log.Printf("%s is %s", envVar, value)
	} else {
		if flagValue != "" {
			log.Printf("using %s \"%s\" from flag", flagVar, flagValue)
			os.Setenv(envVar, flagValue)
		} else {
			return fmt.Errorf("%s is not set from flag nor env %s: %w", flagVar, envVar, err)
		}
	}

	return nil
}

func (s *LoadFlags) Postvalidate() error {
	return nil
}

func (s *LoadFlags) Stop() error {
	return nil
}
