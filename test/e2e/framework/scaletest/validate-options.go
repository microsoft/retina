package scaletest

import (
	"errors"
	"log"
)

type ValidateAndPrintOptions struct {
	Options *Options
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (po *ValidateAndPrintOptions) Prevalidate() error {
	if po.Options.MaxKwokPodsPerNode < 0 ||
		po.Options.NumKwokDeployments < 0 ||
		po.Options.NumKwokReplicas < 0 ||
		po.Options.MaxRealPodsPerNode < 0 ||
		po.Options.NumRealDeployments < 0 ||
		po.Options.NumRealReplicas < 0 ||
		po.Options.NumNetworkPolicies < 0 ||
		po.Options.NumUnapliedNetworkPolicies < 0 ||
		po.Options.NumUniqueLabelsPerPod < 0 ||
		po.Options.NumUniqueLabelsPerDeployment < 0 ||
		po.Options.NumSharedLabelsPerPod < 0 {
		return errors.New("invalid negative value option for Scale step")
	}

	if po.Options.NumNetworkPolicies > 0 && po.Options.NumSharedLabelsPerPod < 3 {
		return errors.New("NumSharedLabelsPerPod must be at least 3 when NumNetworkPolicies > 0 because of the way Network Policies are generated")
	}

	return nil
}

// Returning an error will cause the test to fail
func (po *ValidateAndPrintOptions) Run() error {

	log.Printf("Starting to scale with folowing options: %+v", po.Options)

	return nil
}

// Require for background steps
func (po *ValidateAndPrintOptions) Stop() error {
	return nil
}
