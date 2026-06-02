package flow

import (
	"context"
	"fmt"
	"time"

	ossK8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/pkg/errors"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	sleepDelay = 5 * time.Second
)

type CurlPod struct {
	SrcPodName         string
	SrcPodNamespace    string
	DstPodName         string
	DstPodNamespace    string
	KubeConfigFilePath string
}

func (c *CurlPod) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}
	// Get dst pod IP
	dstPodIP, err := ossK8s.GetPodIP(c.KubeConfigFilePath, c.DstPodNamespace, c.DstPodName)
	if err != nil {
		return errors.Wrap(err, "error getting pod IP")
	}
	cmd := fmt.Sprintf("curl -s -m 5 %s:80", dstPodIP)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = ossK8s.ExecPod(ctx, clientset, config, c.SrcPodNamespace, c.SrcPodName, cmd)
	if err != nil {
		return errors.Wrap(err, "error executing command")
	}
	return nil
}

func (c *CurlPod) Prevalidate() error {
	return nil
}

func (c *CurlPod) Stop() error {
	return nil
}
