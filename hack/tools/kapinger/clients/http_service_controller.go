package clients

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/exp/rand"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type ServiceLoggingController struct {
	sync.RWMutex
	serviceInformer cache.SharedIndexInformer

	ips map[string]string
}

func (c *ServiceLoggingController) Run(ctx context.Context) error {
	stopCh := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(stopCh)
	}()
	c.serviceInformer.Run(stopCh)
	if !cache.WaitForCacheSync(ctx.Done(), c.serviceInformer.HasSynced) {
		return fmt.Errorf("failed to sync")
	}
	return nil
}

func (c *ServiceLoggingController) serviceAdd(obj interface{}) {
	service := obj.(*v1.Service)
	klog.Infof("SERVICE CREATED: %s/%s", service.Namespace, service.Name)
	c.Lock()
	defer c.Unlock()
	c.ips[service.Name] = service.Spec.ClusterIP
}

func (c *ServiceLoggingController) serviceUpdate(old, new interface{}) {
	newsvc := new.(*v1.Service)
	oldsvc := new.(*v1.Service)
	klog.Infof("SERVICE UPDATED. %s/%s", newsvc.Namespace, newsvc.Name)
	c.Lock()
	defer c.Unlock()
	delete(c.ips, oldsvc.Name)
	c.ips[newsvc.Name] = newsvc.Spec.ClusterIP
}

func (c *ServiceLoggingController) serviceDelete(obj interface{}) {
	service := obj.(*v1.Service)
	klog.Infof("SERVICE DELETED: %s/%s", service.Namespace, service.Name)
	c.Lock()
	defer c.Unlock()
	delete(c.ips, service.Name)
}

func (c *ServiceLoggingController) getIP() string {
	c.RLock()
	defer c.RUnlock()
	// select random ip from map
	randIndex := rand.Intn(len(c.ips))
	for key := range c.ips {
		randIndex--
		if randIndex == 0 {
			return c.ips[key]
		}
	}
	return ""
}

func NewServiceLoggingController(clientset kubernetes.Interface, labelselector string) (*ServiceLoggingController, error) {
	serviceInformer := informers.NewSharedInformerFactoryWithOptions(clientset, time.Hour*24,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = labelselector // Filter by label selector
		}),
	).Core().V1().Services().Informer()
	c := &ServiceLoggingController{
		serviceInformer: serviceInformer,
	}
	serviceInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.serviceAdd,
			UpdateFunc: c.serviceUpdate,
			DeleteFunc: c.serviceDelete,
		},
	)

	c.ips = make(map[string]string)
	return c, nil
}
