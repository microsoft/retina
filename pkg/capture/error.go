// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import "fmt"

var _ error = SecretNotFoundError{}

type SecretNotFoundError struct {
	SecretName string
	Namespace  string
}

func (err SecretNotFoundError) Error() string {
	return fmt.Sprintf("secret of Capture is not found when blobupload is enabled, secretName: %s/%s", err.SecretName, err.Namespace)
}

// define a error struct for capture job number limit
var _ error = CaptureJobNumExceedLimitError{}

type CaptureJobNumExceedLimitError struct {
	CurrentNum int
	Limit      int
}

func (err CaptureJobNumExceedLimitError) Error() string {
	return fmt.Sprintf("the number of capture jobs %d exceeds the limit %d", err.CurrentNum, err.Limit)
}
