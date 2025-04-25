package common

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

var (
	clusterName   = os.Getenv("CLUSTER_NAME")
	subID         = os.Getenv("AZURE_SUBSCRIPTION_ID")
	location      = os.Getenv("AZURE_LOCATION")
	resourceGroup = os.Getenv("AZURE_RESOURCE_GROUP")

	errAzureSubscriptionIDNotSet = errors.New("AZURE_SUBSCRIPTION_ID is not set")
)

var (
	locations     = []string{"eastus2", "centralus", "southcentralus", "uksouth", "centralindia", "westus2"}
	architectures = []string{"amd64", "arm64"}
	createInfra   = flag.Bool("create-infra", true, "create a Resource group, vNET and AKS cluster for testing")
	deleteInfra   = flag.Bool("delete-infra", true, "delete a Resource group, vNET and AKS cluster for testing")
)

type TestInfraSettings struct {
	CreateInfra         bool
	DeleteInfra         bool
	ChartPath           string
	ProfilePath         string
	AdvancedProfilePath string
	KubeConfigFilePath  string

	ClusterName   string
	SubID         string
	Location      string
	ResourceGroup string

	AzureLocations []string
	Architectures  []string
}

func LoadInfraSettings() (*TestInfraSettings, error) {
	curuser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Failed to get current user: %w", err)
	}

	flag.Parse()

	if clusterName == "" {
		clusterName = curuser.Username + NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)
		log.Printf("CLUSTER_NAME is not set, generating a random cluster name: %s", clusterName)
	}

	if subID == "" {
		return nil, errAzureSubscriptionIDNotSet
	}

	if location == "" {
		var nBig *big.Int
		nBig, err = rand.Int(rand.Reader, big.NewInt(int64(len(locations))))
		if err != nil {
			return nil, fmt.Errorf("Failed to generate a secure random index: %w", err)
		}
		location = locations[nBig.Int64()]
	}

	if resourceGroup == "" {
		log.Printf("AZURE_RESOURCE_GROUP is not set, using the cluster name as the resource group name by default")
		resourceGroup = clusterName
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Failed to get current working directory: %w", err)
	}

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	advancedProfilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	return &TestInfraSettings{
		CreateInfra:         *createInfra,
		DeleteInfra:         *deleteInfra,
		ChartPath:           chartPath,
		ProfilePath:         profilePath,
		AdvancedProfilePath: advancedProfilePath,
		KubeConfigFilePath:  kubeConfigFilePath,
		ClusterName:         clusterName,
		SubID:               subID,
		Location:            location,
		ResourceGroup:       resourceGroup,
		AzureLocations:      locations,
		Architectures:       architectures,
	}, nil
}
