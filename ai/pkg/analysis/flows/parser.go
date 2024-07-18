package flows

import (
	"fmt"
	"sort"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	"github.com/sirupsen/logrus"
)

type Parser struct {
	log     logrus.FieldLogger
	summary FlowSummary
}

func NewParser(log logrus.FieldLogger) *Parser {
	return &Parser{
		log:     log.WithField("component", "flow-parser"),
		summary: make(map[string]*Connection),
	}
}

func (p *Parser) Summary() FlowSummary {
	return p.summary
}

func (p *Parser) Parse(flows []*flowpb.Flow) {
	for _, flow := range flows {
		err := p.addFlow(flow)
		if err != nil {
			p.log.WithError(err).WithField("flow", flow).Error("failed to add flow")
		}
	}
}

func (p *Parser) addFlow(f *flowpb.Flow) error {
	src := f.GetSource()
	dst := f.GetDestination()
	if src == nil || dst == nil {
		// empty flow. ignore
		return nil
	}

	srcName, err := endpointName(src)
	if err != nil {
		return fmt.Errorf("error getting source name: %w", err)
	}

	dstName, err := endpointName(dst)
	if err != nil {
		return fmt.Errorf("error getting destination name: %w", err)
	}

	// Ensure pod1 is alphabetically before pod2
	pods := []string{srcName, dstName}
	sort.Strings(pods)
	pod1, pod2 := pods[0], pods[1]
	key := pod1 + "#" + pod2

	conn, exists := p.summary[key]
	if !exists {
		conn = &Connection{
			Pod1:  pod1,
			Pod2:  pod2,
			Key:   key,
			Flows: []*flowpb.Flow{},
		}
		p.summary[key] = conn
	}

	conn.Flows = append(conn.Flows, f)
	return nil
}

func endpointName(ep *flowpb.Endpoint) (string, error) {
	name := ep.GetPodName()
	if name != "" {
		return name, nil
	}

	lbls := ep.GetLabels()
	if len(lbls) == 0 {
		return "", ErrNoEndpointName
	}
	// should be a reserved label like:
	// reserved:world
	// reserved:host
	// reserved:kube-apiserver
	return ep.GetLabels()[0], nil
}
