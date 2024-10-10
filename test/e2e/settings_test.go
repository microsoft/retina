package retina

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
)

var (
	clusterName = os.Getenv("CLUSTER_NAME")
	subID       = os.Getenv("AZURE_SUBSCRIPTION_ID")
	location    = os.Getenv("AZURE_LOCATION")
)

var (
	locations   = []string{"eastus2", "centralus", "southcentralus", "uksouth", "centralindia", "westus2"}
	createInfra = flag.Bool("create-infra", true, "create a Resource group, vNET and AKS cluster for testing")
	deleteInfra = flag.Bool("delete-infra", true, "delete a Resource group, vNET and AKS cluster for testing")
)

type TestInfraSettings struct {
	CreateInfra        bool
	DeleteInfra        bool
	ChartPath          string
	ProfilePath        string
	KubeConfigFilePath string
}

func LoadInfraSettings() (*TestInfraSettings, error) {
	curuser, err := user.Current()
	if err != nil {
		return nil, err
	}

	flag.Parse()

	if clusterName == "" {
		clusterName = curuser.Username + common.NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)
		log.Printf("CLUSTER_NAME is not set, generating a random cluster name: %s", clusterName)
	}

	if subID == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID is not set")
	}

	if location == "" {
		var nBig *big.Int
		nBig, err = rand.Int(rand.Reader, big.NewInt(int64(len(locations))))
		if err != nil {
			return nil, fmt.Errorf("Failed to generate a secure random index: %v", err)
		}
		location = locations[nBig.Int64()]
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	return &TestInfraSettings{
		CreateInfra:        *createInfra,
		DeleteInfra:        *deleteInfra,
		ChartPath:          chartPath,
		ProfilePath:        profilePath,
		KubeConfigFilePath: kubeConfigFilePath,
	}, nil
}
