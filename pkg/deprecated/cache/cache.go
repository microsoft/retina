// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmgr "sigs.k8s.io/controller-runtime/pkg/manager"
)

type Cache struct {
	// local cache containing services + pods
	objCache map[string]client.Object
	// informer to track pod events
	podInformer v1.PodInformer
	// informer to track service events
	svcInformer v1.ServiceInformer
	// mgr mostly used to start the cache in this pkg
	mgr             crmgr.Manager
	informerFactory informers.SharedInformerFactory
	logger          *log.ZapLogger
}

const (
	ResyncTime time.Duration = 5 * time.Minute
)

func (c *Cache) add(obj interface{}) {
	cObj, ok := obj.(client.Object)
	if !ok {
		c.logger.Error("Failed to cast to Kubernetes client object")
		return
	}
	objIP, err := GetObjectIP(cObj)
	if err != nil {
		c.logger.Error("Failed to get IP", zap.Error(err))
		return
	}
	if len(objIP) > 0 {
		c.objCache[objIP] = cObj
	}
}

func (c *Cache) update(old, new interface{}) {
	oObj, ook := old.(client.Object)
	nObj, nok := new.(client.Object)
	if !ook || !nok {
		c.logger.Error("Failed to cast to Kubernetes client object")
		return
	}
	oObjIP, oerr := GetObjectIP(oObj)
	nObjIP, nerr := GetObjectIP(nObj)
	if oerr != nil || nerr != nil {
		c.logger.Error("Failed to get IP", zap.Error(oerr), zap.Error(nerr))
		return
	}
	if len(nObjIP) > 0 {
		if len(oObjIP) > 0 && oObjIP != nObjIP {
			delete(c.objCache, oObjIP)
		}
		c.objCache[nObjIP] = nObj
	}
}

func (c *Cache) delete(obj interface{}) {
	cObj, ok := obj.(client.Object)
	if !ok {
		c.logger.Error("Failed to cast to Kubernetes client object")
		return
	}
	objIP, err := GetObjectIP(cObj)
	if err != nil {
		c.logger.Error("Failed to get IP", zap.Error(err))
		return
	}
	delete(c.objCache, objIP)
}

func New(logger *log.ZapLogger, mgr crmgr.Manager, cl kubernetes.Interface, factory informers.SharedInformerFactory) *Cache {
	cacheMap := make(map[string]client.Object)
	return &Cache{
		objCache:        cacheMap,
		mgr:             mgr,
		informerFactory: factory,
		logger:          logger,
	}
}

func (lc *Cache) Start(ctx context.Context) error {
	lc.logger.Info("Starting cache...")
	lc.SetInformers(lc.informerFactory)
	err := lc.mgr.GetCache().Start(ctx)
	return err
}

func (lc *Cache) SetInformers(informerFactory informers.SharedInformerFactory) {
	lc.SetPodInformer(informerFactory)
	lc.SetServiceInformer(informerFactory)
}

func (lc *Cache) SetPodInformer(informerFactory informers.SharedInformerFactory) {
	podInformer := informerFactory.Core().V1().Pods()
	_, err := podInformer.Informer().AddEventHandler(
		&cache.ResourceEventHandlerFuncs{
			AddFunc:    lc.add,
			UpdateFunc: lc.update,
			DeleteFunc: lc.delete,
		},
	)
	if err != nil {
		lc.logger.Error("Failed to add pod event handler", zap.Error(err))
	}
	lc.podInformer = podInformer
}

func (lc *Cache) SetServiceInformer(informerFactory informers.SharedInformerFactory) {
	svcInformer := informerFactory.Core().V1().Services()
	lc.svcInformer = svcInformer
	_, err := svcInformer.Informer().AddEventHandler(
		&cache.ResourceEventHandlerFuncs{
			AddFunc:    lc.add,
			UpdateFunc: lc.update,
			DeleteFunc: lc.delete,
		},
	)
	if err != nil {
		lc.logger.Error("Failed to add service event handler", zap.Error(err))
	}
}

func (c *Cache) LookupObjectByIP(ip string) (client.Object, error) {
	obj, ok := c.objCache[ip]
	if ok {
		return obj, nil
	}
	// check cache podlist
	var podList corev1.PodList
	obj, _ = c.CacheLookupObjectByIP(ip, &podList)
	if obj != nil {
		return obj, nil
	}
	// check cache service list
	var svcList corev1.ServiceList
	obj, _ = c.CacheLookupObjectByIP(ip, &svcList)
	if obj != nil {
		return obj, nil
	}
	return nil, errors.New("Pod or service not found")
}

func (c *Cache) CacheLookupObjectByIP(ip string, list client.ObjectList) (client.Object, error) {
	err := c.mgr.GetCache().List(context.TODO(), list)
	if err != nil {
		return nil, err
	}
	switch list.(type) {
	case *corev1.PodList:
		return c.LookupPodByIP(list, ip)
	case *corev1.ServiceList:
		return c.LookupServiceByIP(list, ip)
	default:
		return nil, errors.New("List type not supported")
	}
}

func (c *Cache) LookupServiceByIP(list client.ObjectList, ip string) (client.Object, error) {
	v, ok := list.(*corev1.ServiceList)
	if !ok {
		return nil, errors.New("Object is not a Service List")
	}
	for _, obj := range v.Items {
		if oip, err := GetObjectIP(&obj); err == nil && (oip == ip) {
			c.add(&obj)
			return c.objCache[ip], nil
		}
	}
	return nil, errors.New("Service not found")
}

func (c *Cache) LookupPodByIP(list client.ObjectList, ip string) (client.Object, error) {
	v, ok := list.(*corev1.PodList)
	if !ok {
		return nil, errors.New("Object is not a PodList")
	}
	for _, obj := range v.Items {
		if oip, err := GetObjectIP(&obj); err == nil && (oip == ip) {
			c.add(&obj)
			return c.objCache[ip], nil
		}
	}
	return nil, errors.New("Pod not found")
}

func (c *Cache) GetPodOwner(obj interface{}) (string, string) {
	var name, kind string
	switch p := obj.(type) {
	case *corev1.Pod:
		if len(p.OwnerReferences) == 0 {
			return name, kind
		}
		name = p.OwnerReferences[0].Name
		switch v := p.OwnerReferences[0].Kind; v {
		case "DaemonSet", "StatefulSet":
			kind = v
		case "ReplicaSet":
			kind = v
			rs := &appsv1.ReplicaSet{}
			rk := types.NamespacedName{
				Namespace: p.Namespace,
				Name:      name,
			}
			err := c.mgr.GetClient().Get(context.TODO(), rk, rs)
			if err != nil {
				c.logger.Warn("Error finding replicaset", zap.Error(err))
			}
			if len(rs.OwnerReferences) > 0 {
				kind = "Deployment"
				name = rs.OwnerReferences[0].Name
			}
		}
	}
	return name, kind
}

func GetObjectIP(obj interface{}) (string, error) {
	switch v := obj.(type) {
	case *corev1.Service:
		return v.Spec.ClusterIP, nil
	case *corev1.Pod:
		return v.Status.PodIP, nil
	}
	return "", errors.New("Non supported type")
}
