package mock

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Verify interface compliance at compile time
var _ manager.Manager = (*controllerManager)(nil)

type controllerManager struct {
	manager.Manager
}

func NewControllerManager() manager.Manager {
	return &controllerManager{}
}

func (m *controllerManager) Add(manager.Runnable) error {
	return nil
}

func (m *controllerManager) GetCache() cache.Cache {
	return nil
}

func (m *controllerManager) GetControllerOptions() config.Controller {
	return config.Controller{}
}

func (m *controllerManager) GetScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	return scheme
}

func (m *controllerManager) GetLogger() logr.Logger {
	return logr.Discard()
}
