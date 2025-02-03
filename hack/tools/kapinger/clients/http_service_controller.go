package clients

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"log"

	"golang.org/x/exp/rand"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type serviceController struct {
	sync.RWMutex
	serviceInformer cache.SharedIndexInformer

	ips []net.IP
}

func newServiceController(clientset kubernetes.Interface, labelselector string) (*serviceController, error) {
	serviceInformer := informers.NewSharedInformerFactoryWithOptions(clientset, time.Hour*24,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = labelselector // Filter by label selector
		}),
	).Core().V1().Services().Informer()
	c := &serviceController{
		serviceInformer: serviceInformer,
	}
	serviceInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.serviceAdd,
			UpdateFunc: c.serviceUpdate,
			DeleteFunc: c.serviceDelete,
		},
	)

	return c, nil
}

func (c *serviceController) run(ctx context.Context) error {
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

func (c *serviceController) serviceAdd(obj interface{}) {
	service := obj.(*v1.Service)
	log.Printf("service %s/%s added with ip %s", service.Namespace, service.Name, service.Spec.ClusterIP)
	c.addIP(net.ParseIP(service.Spec.ClusterIP))
}

func (c *serviceController) serviceUpdate(old, new interface{}) {
	newsvc := new.(*v1.Service)
	oldsvc := new.(*v1.Service)
	log.Printf("service %s/%s updated with new ip %s", newsvc.Namespace, newsvc.Name, newsvc.Spec.ClusterIP)
	c.removeIP(net.ParseIP(oldsvc.Spec.ClusterIP))
	c.addIP(net.ParseIP(newsvc.Spec.ClusterIP))
}

func (c *serviceController) serviceDelete(obj interface{}) {
	service := obj.(*v1.Service)
	log.Printf("service %s/%s deleted", service.Namespace, service.Name)
	c.removeIP(net.ParseIP(service.Spec.ClusterIP))
}

func (c *serviceController) getIP() net.IP {
	c.RLock()
	defer c.RUnlock()
	return c.ips[rand.Intn(len(c.ips))]
}

func (c *serviceController) addIP(ip net.IP) {
	c.Lock()
	defer c.Unlock()
	c.ips = append(c.ips, ip)
}

func (c *serviceController) removeIP(ip net.IP) {
	c.Lock()
	defer c.Unlock()

	// find the index of the ip
	i := -1
	for j, cip := range c.ips {
		if cip.Equal(ip) {
			i = j
			break
		}
	}
	if i == -1 {
		log.Printf("service with ip %s not found", ip)
		return
	}

	c.ips = slices.Delete(c.ips, i, i+1)
	c.ips = slices.Clip(c.ips)
}
