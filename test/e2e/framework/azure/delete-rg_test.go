package azure

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/stretchr/testify/require"
)

const transientDeletionBlockedErrorCode = "ResourceGroupDeletionBlocked"

func TestDeleteResourceGroupsDeletesParentAndNodeResourceGroups(t *testing.T) {
	deleted := map[string]bool{}
	var deletionOrder []string
	operations := resourceGroupDeletionOperations{
		getNodeResourceGroup: func(context.Context) (string, error) {
			return "MC_test", nil
		},
		resourceGroupExists: func(_ context.Context, resourceGroupName string) (bool, error) {
			return !deleted[resourceGroupName], nil
		},
		beginDelete: func(_ context.Context, resourceGroupName string) (pollResourceGroupDeletion, error) {
			deletionOrder = append(deletionOrder, resourceGroupName)
			return func(context.Context) error {
				deleted[resourceGroupName] = true
				return nil
			}, nil
		},
		wait: noWait,
	}

	err := deleteResourceGroups(context.Background(), "test", operations)

	require.NoError(t, err)
	require.Equal(t, []string{"test", "MC_test"}, deletionOrder)
}

func TestDeleteResourceGroupRetriesTransientPollingFailure(t *testing.T) {
	attempts := 0
	operations := resourceGroupDeletionOperations{
		resourceGroupExists: alwaysResourceGroupExists,
		beginDelete: func(context.Context, string) (pollResourceGroupDeletion, error) {
			attempts++
			return func(context.Context) error {
				if attempts == 1 {
					return &azcore.ResponseError{
						ErrorCode:  transientDeletionBlockedErrorCode,
						StatusCode: http.StatusConflict,
					}
				}
				return nil
			}, nil
		},
		wait: noWait,
	}

	err := deleteResourceGroupWithRetry(context.Background(), "test", operations)

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
}

func TestDeleteResourceGroupRetriesTransientBeginFailure(t *testing.T) {
	attempts := 0
	operations := resourceGroupDeletionOperations{
		resourceGroupExists: alwaysResourceGroupExists,
		beginDelete: func(context.Context, string) (pollResourceGroupDeletion, error) {
			attempts++
			if attempts == 1 {
				return nil, &azcore.ResponseError{
					ErrorCode:  "AnotherOperationInProgress",
					StatusCode: http.StatusConflict,
				}
			}
			return func(context.Context) error { return nil }, nil
		},
		wait: noWait,
	}

	err := deleteResourceGroupWithRetry(context.Background(), "test", operations)

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
}

func TestDeleteResourceGroupReturnsNonTransientFailureWithoutRetry(t *testing.T) {
	attempts := 0
	operations := resourceGroupDeletionOperations{
		resourceGroupExists: alwaysResourceGroupExists,
		beginDelete: func(context.Context, string) (pollResourceGroupDeletion, error) {
			attempts++
			return nil, &azcore.ResponseError{
				ErrorCode:  "AuthorizationFailed",
				StatusCode: http.StatusForbidden,
			}
		},
		wait: noWait,
	}

	err := deleteResourceGroupWithRetry(context.Background(), "test", operations)

	require.ErrorContains(t, err, "AuthorizationFailed")
	require.Equal(t, 1, attempts)
}

func TestDeleteResourceGroupReturnsLastFailureWhenRetryDeadlineExpires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	operations := resourceGroupDeletionOperations{
		resourceGroupExists: alwaysResourceGroupExists,
		beginDelete: func(context.Context, string) (pollResourceGroupDeletion, error) {
			return nil, &azcore.ResponseError{
				ErrorCode:  transientDeletionBlockedErrorCode,
				StatusCode: http.StatusConflict,
			}
		},
		wait: func(context.Context, time.Duration) error {
			cancel()
			return context.DeadlineExceeded
		},
	}

	err := deleteResourceGroupWithRetry(ctx, "test", operations)

	require.ErrorContains(t, err, "timed out waiting to retry")
	require.ErrorContains(t, err, "ResourceGroupDeletionBlocked")
}

func TestDeleteResourceGroupLogsAcceptanceBeforeCompletion(t *testing.T) {
	var logs bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})

	operations := resourceGroupDeletionOperations{
		resourceGroupExists: alwaysResourceGroupExists,
		beginDelete: func(context.Context, string) (pollResourceGroupDeletion, error) {
			return func(context.Context) error { return nil }, nil
		},
		wait: noWait,
	}

	err := deleteResourceGroupWithRetry(context.Background(), "test", operations)
	require.NoError(t, err)

	output := logs.String()
	acceptedIndex := strings.Index(output, `Azure accepted deletion of resource group "test"`)
	completedIndex := strings.Index(output, `resource group "test" deleted successfully`)
	require.NotEqual(t, -1, acceptedIndex)
	require.NotEqual(t, -1, completedIndex)
	require.Less(t, acceptedIndex, completedIndex)
}

func TestIsTransientResourceGroupDeletionError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name: "wrapped deletion blocked",
			err: fmt.Errorf("poll failed: %w", &azcore.ResponseError{
				ErrorCode:  transientDeletionBlockedErrorCode,
				StatusCode: http.StatusConflict,
			}),
			transient: true,
		},
		{
			name: "throttled",
			err: &azcore.ResponseError{
				ErrorCode:  "TooManyRequests",
				StatusCode: http.StatusTooManyRequests,
			},
			transient: true,
		},
		{
			name: "unknown conflict",
			err: &azcore.ResponseError{
				ErrorCode:  "Conflict",
				StatusCode: http.StatusConflict,
			},
			transient: false,
		},
		{
			name: "bad request",
			err: &azcore.ResponseError{
				ErrorCode:  "InvalidRequest",
				StatusCode: http.StatusBadRequest,
			},
			transient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.transient, isTransientResourceGroupDeletionError(tt.err))
		})
	}
}

func alwaysResourceGroupExists(context.Context, string) (bool, error) {
	return true, nil
}

func noWait(context.Context, time.Duration) error {
	return nil
}
