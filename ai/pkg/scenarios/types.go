package scenarios

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/retina/ai/pkg/lm"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Definition struct {
	Name        string
	Description string
	Specs       []*ParameterSpec
	handler
}

func NewDefinition(name, description string, specs []*ParameterSpec, handler handler) *Definition {
	return &Definition{
		Name:        name,
		Description: description,
		Specs:       specs,
		handler:     handler,
	}
}

func (d *Definition) Handle(ctx context.Context, cfg *Config, rawParams map[string]string, question string, history lm.ChatHistory) (string, error) {
	typedParams := make(map[string]any)

	// validate params
	for _, p := range d.Specs {
		raw, ok := rawParams[p.Name]
		if !ok {
			if !p.Optional {
				return "", fmt.Errorf("missing required parameter %s", p.Name)
			}

			continue
		}

		if p.Regex != nil && !p.Regex.MatchString(raw) {
			return "", fmt.Errorf("parameter %s does not match regex format", p.Name)
		}

		switch p.DataType {
		case "string":
			typedParams[p.Name] = raw
		case "int":
			i, err := strconv.Atoi(raw)
			if err != nil {
				return "", fmt.Errorf("parameter %s is not an integer", p.Name)
			}
			typedParams[p.Name] = i
		case "[]string":
			// make sure the format is like [a,b,c]
			if raw == "" || raw[0] != '[' || raw[len(raw)-1] != ']' || strings.Count(raw, "[") != 1 || strings.Count(raw, "]") != 1 {
				return "", fmt.Errorf("invalid array format for parameter %s", p.Name)
			}
			// remove brackets
			raw = raw[1 : len(raw)-1]
			typedParams[p.Name] = strings.Split(raw, ",")
		default:
			return "", fmt.Errorf("unsupported data type %s", p.DataType)
		}
	}

	return d.handler.Handle(ctx, cfg, typedParams, question, history)
}

type ParameterSpec struct {
	Name        string
	DataType    string
	Description string
	Optional    bool
	Regex       *regexp.Regexp
}

type handler interface {
	Handle(ctx context.Context, cfg *Config, typedParams map[string]any, question string, history lm.ChatHistory) (string, error)
}

type Config struct {
	Log           logrus.FieldLogger
	Config        *rest.Config
	Clientset     *kubernetes.Clientset
	Model         lm.Model
	FlowRetriever *flowretrieval.Retriever
}
