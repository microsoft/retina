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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const BlobURL = "BLOB_URL"

var ErrEmptyBlobURL = errors.New("BLOB_URL must be set/exported")

var downloadCapture = &cobra.Command{
	Use:   "download",
	Short: "Download Retina Captures",
	RunE: func(*cobra.Command, []string) error {
		blobURL := viper.GetString(BlobURL)
		if blobURL == "" {
			return ErrEmptyBlobURL
		}

		u, err := url.Parse(blobURL)
		if err != nil {
			return errors.Wrapf(err, "failed to parse SAS URL %s", blobURL)
		}

		// blobService, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode()).GetBlobService()
		b, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode())
		if err != nil {
			return errors.Wrap(err, "failed to create storage account client")
		}

		blobService := b.GetBlobService()
		containerPath := strings.TrimLeft(u.Path, "/")
		splitPath := strings.SplitN(containerPath, "/", 2) //nolint:gomnd // TODO string splitting probably isn't the right way to parse this URL?
		containerName := splitPath[0]

		params := storage.ListBlobsParameters{Prefix: *opts.Name}
		blobList, err := blobService.GetContainerReference(containerName).ListBlobs(params)
		if err != nil {
			return errors.Wrap(err, "failed to list blobstore ")
		}

		if len(blobList.Blobs) == 0 {
			return errors.Errorf("no blobs found with prefix: %s", *opts.Name)
		}

		for _, v := range blobList.Blobs {
			blob := blobService.GetContainerReference(containerName).GetBlobReference(v.Name)
			readCloser, err := blob.Get(&storage.GetBlobOptions{})
			if err != nil {
				return errors.Wrap(err, "failed to read from blobstore")
			}

			defer readCloser.Close()

			blobData, err := io.ReadAll(readCloser)
			if err != nil {
				return errors.Wrap(err, "failed to obtain blob from blobstore")
			}

			err = os.WriteFile(v.Name, blobData, 0o644) //nolint:gosec,gomnd // intentionally permissive bitmask
			if err != nil {
				return errors.Wrap(err, "failed to write file")
			}
			fmt.Println("Downloaded blob: ", v.Name)
		}
		return nil
	},
}

func init() {
	capture.AddCommand(downloadCapture)
}
