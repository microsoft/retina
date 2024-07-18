package flows

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/microsoft/retina/ai/pkg/retrieval/flows/client"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/cilium/hubble-ui/backend/domain/labels"
	"github.com/cilium/hubble-ui/backend/domain/service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Retriever struct {
	log         logrus.FieldLogger
	config      *rest.Config
	clientset   *kubernetes.Clientset
	initialized bool
	client      *client.Client
	flows       []*flowpb.Flow
}

func NewRetriever(log logrus.FieldLogger, config *rest.Config, clientset *kubernetes.Clientset) *Retriever {
	return &Retriever{
		log:       log.WithField("component", "flow-retriever"),
		config:    config,
		clientset: clientset,
	}
}

func (r *Retriever) Init() error {
	client, err := client.New()
	if err != nil {
		return fmt.Errorf("failed to create grpc client. %v", err)
	}

	r.log.Info("initialized grpc client")

	r.client = client
	r.initialized = true
	return nil
}

func (r *Retriever) Observe(ctx context.Context, maxFlows int) ([]*flowpb.Flow, error) {
	if !r.initialized {
		if err := r.Init(); err != nil {
			return nil, fmt.Errorf("failed to initialize. %v", err)
		}
	}

	// translate parameters to flow request
	// TODO: use parameters
	req := flowsRequest()

	// port-forward to hubble-relay
	portForwardCtx, portForwardCancel := context.WithCancel(ctx)
	defer portForwardCancel()

	// FIXME make ports part of a config
	cmd := exec.CommandContext(portForwardCtx, "kubectl", "port-forward", "-n", "kube-system", "svc/hubble-relay", "5555:80")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward. %v", err)
	}

	// observe flows
	observeCtx, observeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer observeCancel()
	flows, err := r.observeFlowsGRPC(observeCtx, req, maxFlows)
	if err != nil {
		return nil, fmt.Errorf("failed to observe flows over grpc. %v", err)
	}

	// stop the port-forward
	portForwardCancel()
	// will error with "exit status 1" because of context cancellation
	_ = cmd.Wait()
	r.log.Info("stopped port-forward")

	return flows, nil
}

func flowsRequest() *observerpb.GetFlowsRequest {
	return &observerpb.GetFlowsRequest{
		Number:    200,
		Follow:    false,
		Whitelist: []*flowpb.FlowFilter{},
		Blacklist: nil,
		Since:     nil,
		Until:     nil,
		First:     false,
	}
}

func (r *Retriever) observeFlowsGRPC(ctx context.Context, req *observerpb.GetFlowsRequest, maxFlows int) ([]*flowpb.Flow, error) {
	stream, err := r.client.GetFlows(ctx, req, grpc.WaitForReady(true))
	if err != nil {
		return nil, fmt.Errorf("failed to get flow stream. %v", err)
	}

	r.flows = make([]*flowpb.Flow, 0)
	for {
		select {
		case <-ctx.Done():
			r.log.Info("context cancelled")
			return r.flows, nil
		default:
			r.log.WithField("flowCount", len(r.flows)).Debug("processing flow")

			getFlowResponse, err := stream.Recv()
			if err != nil {
				// TODO handle error instead of returning error
				return nil, fmt.Errorf("failed to receive flow. %v", err)
			}

			f := getFlowResponse.GetFlow()
			if f == nil {
				continue
			}

			r.handleFlow(f)
			if len(r.flows) >= maxFlows {
				return r.flows, nil
			}
		}
	}
}

// handleFlow logic is inspired by a snippet from Hubble UI
// https://github.com/cilium/hubble-ui/blob/a06e19ba65299c63a58034a360aeedde9266ec01/backend/internal/flow_stream/flow_stream.go#L360-L395
func (r *Retriever) handleFlow(f *flowpb.Flow) {
	if f.GetL4() == nil || f.GetSource() == nil || f.GetDestination() == nil {
		return
	}

	sourceId, destId := service.IdsFromFlowProto(f)
	if sourceId == "0" || destId == "0" {
		r.log.Warn("invalid (zero) identity in source / dest services")
		// TODO print offending flow?
		return
	}

	// TODO: workaround to hide flows/services which are showing as "World",
	// but actually they are k8s services without initialized pods.
	// Appropriate fix is to construct and show special service map cards
	// and show these flows in special way inside flows table.
	if f.GetDestination() != nil {
		destService := f.GetDestinationService()
		destLabelsProps := labels.Props(f.GetDestination().GetLabels())
		destNames := f.GetDestinationNames()
		isDestOutside := destLabelsProps.IsWorld || len(destNames) > 0

		if destService != nil && isDestOutside {
			return
		}
	}

	r.flows = append(r.flows, f)
}
