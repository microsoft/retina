package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

const MaxFlowsFromHubbleRelay = 30000

type Retriever struct {
	log          logrus.FieldLogger
	config       *rest.Config
	clientset    *kubernetes.Clientset
	initialized  bool
	client       *client.Client
	readFromFile bool
	flows        []*flowpb.Flow
}

func NewRetriever(log logrus.FieldLogger, config *rest.Config, clientset *kubernetes.Clientset) *Retriever {
	return &Retriever{
		log:       log.WithField("component", "flow-retriever"),
		config:    config,
		clientset: clientset,
	}
}

func (r *Retriever) UseFile() {
	r.readFromFile = true
}

func (r *Retriever) Init() error {
	if r.readFromFile {
		r.log.Info("using flows from file")
		return nil
	}

	client, err := client.New()
	if err != nil {
		return fmt.Errorf("failed to create grpc client. %v", err)
	}

	r.log.Info("initialized grpc client")

	r.client = client
	r.initialized = true
	return nil
}

func (r *Retriever) Observe(ctx context.Context, req *observerpb.GetFlowsRequest) ([]*flowpb.Flow, error) {
	if r.readFromFile {
		flows, err := readFlowsFromFile("flows.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read flows from file. %v", err)
		}

		return flows, nil
	}

	if !r.initialized {
		if err := r.Init(); err != nil {
			return nil, fmt.Errorf("failed to initialize. %v", err)
		}
	}

	// port-forward to hubble-relay
	portForwardCtx, portForwardCancel := context.WithCancel(ctx)
	defer portForwardCancel()

	// FIXME make ports part of a config
	cmd := exec.CommandContext(portForwardCtx, "kubectl", "port-forward", "-n", "kube-system", "svc/hubble-relay", "5557:80")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward. %v", err)
	}

	// observe flows
	observeCtx, observeCancel := context.WithTimeout(ctx, 15*time.Second)
	defer observeCancel()

	maxFlows := req.Number
	flows, err := r.observeFlowsGRPC(observeCtx, req, int(maxFlows))
	if err != nil {
		return nil, fmt.Errorf("failed to observe flows over grpc. %v", err)
	}

	// stop the port-forward
	portForwardCancel()
	// will error with "exit status 1" because of context cancellation
	_ = cmd.Wait()
	r.log.Info("stopped port-forward")

	r.log.Info("saving flows to JSON")
	if err := saveFlowsToJSON(flows, "flows.json"); err != nil {
		r.log.WithError(err).Error("failed to save flows to JSON")
		return nil, err
	}

	return flows, nil
}

func (r *Retriever) observeFlowsGRPC(ctx context.Context, req *observerpb.GetFlowsRequest, maxFlows int) ([]*flowpb.Flow, error) {
	stream, err := r.client.GetFlows(ctx, req, grpc.WaitForReady(true))
	if err != nil {
		return nil, fmt.Errorf("failed to get flow stream. %v", err)
	}

	r.flows = make([]*flowpb.Flow, 0)
	var errReceiving error
	for {
		select {
		case <-ctx.Done():
			r.log.Info("context cancelled")
			return r.flows, nil
		default:
			if errReceiving != nil {
				// error receiving and context not done
				// TODO handle error instead of returning error
				return nil, fmt.Errorf("failed to receive flow. %v", err)
			}

			r.log.WithField("flowCount", len(r.flows)).Debug("processing flow")

			getFlowResponse, err := stream.Recv()
			if err != nil {
				errReceiving = err
				continue
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
	if (f.GetL7() == nil && f.GetL4() == nil) || f.GetSource() == nil || f.GetDestination() == nil {
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

func saveFlowsToJSON(flows []*flowpb.Flow, filename string) error {
	for _, f := range flows {
		// to avoid getting an error:
		// failed to encode JSON: json: error calling MarshalJSON for type *flow.Flow: proto:\u00a0google.protobuf.Any: unable to resolve \"type.googleapis.com/utils.RetinaMetadata\": not found
		f.Extensions = nil
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // optional: to make the JSON output pretty
	if err := encoder.Encode(flows); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func readFlowsFromFile(filename string) ([]*flowpb.Flow, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var flows []*flowpb.Flow
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&flows); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return flows, nil
}
