// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const BlobURL = "BLOB_URL"

var (
	ErrEmptyBlobURL = fmt.Errorf("BLOB_URL must be set/exported")
	captureName     string
)

var download = &cobra.Command{
	Use:   "download",
	Short: "Download Retina Captures",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := downloadF(cmd, captureName)
		return err
	},
}

func downloadF(cmd *cobra.Command, captureName string) (string, error) {
	blobURL := viper.GetString(BlobURL)
	if blobURL == "" {
		return "", ErrEmptyBlobURL
	}

	bloburl := viper.GetString(BlobURL)
	if bloburl == "" {
		return "", ErrEmptyBlobURL
	}

	u, err := url.Parse(bloburl)
	if err != nil {
		return "", fmt.Errorf("failed to parse SAS URL: %+v", err)
	}

	// blobService, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode()).GetBlobService()
	b, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode())
	if err != nil {
		return "", fmt.Errorf("failed to create storage account client: %+v", err)
	}

	blobService := b.GetBlobService()
	containerPath := strings.TrimLeft(u.Path, "/")
	splitPath := strings.SplitN(containerPath, "/", 2)
	containerName := splitPath[0]

	params := storage.ListBlobsParameters{Prefix: captureName}
	blobList, err := blobService.GetContainerReference(containerName).ListBlobs(params)
	if err != nil {
		return "", fmt.Errorf("failed to list blobstore with: %+v", err)
	}

	if len(blobList.Blobs) == 0 {
		return "", fmt.Errorf("no blobs found with prefix: %s", captureName)
	}

	blobName := ""
	for _, v := range blobList.Blobs {
		blob := blobService.GetContainerReference(containerName).GetBlobReference(v.Name)
		readCloser, err := blob.Get(&storage.GetBlobOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to read from blobstore with: %+v", err)
		}

		defer readCloser.Close()

		blobData, err := io.ReadAll(readCloser)
		if err != nil {
			return "", fmt.Errorf("failed to obtain blob from blobstore with: %+v", err)
		}

		err = os.WriteFile(v.Name, blobData, 0o644)
		if err != nil {
			return "", fmt.Errorf("failed to write file with: %+v", err)
		}

		blobName = v.Name
		fmt.Println("Downloaded blob: ", v.Name)
	}
	return blobName, nil
}

func init() {
	capture.AddCommand(download)
	download.Flags().StringVarP(&captureName, "capture-name", "n", "", "name of capture to download")
}
