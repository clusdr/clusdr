package operator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	resync     = 30 * time.Second
	requeue    = 12 * time.Second
	syncBudget = 45 * time.Second
)

// Run watches ClusdrCluster objects and reconciles until ctx is cancelled.
func Run(ctx context.Context, core kubernetes.Interface, dyn dynamic.Interface, namespace string, joiner Joiner) error {
	if joiner == nil {
		joiner = Runtime{}
	}
	c := &controller{
		dyn:   dyn,
		rec:   &Reconciler{Kube: Kube{Core: core, Dyn: dyn}, Joiner: joiner},
		queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		log:   slog.Default(),
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dyn, resync, namespace, nil)
	inf := factory.ForResource(GVR).Informer()
	_, err := inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.enqueue(obj) },
		UpdateFunc: func(_, obj any) { c.enqueue(obj) },
	})
	if err != nil {
		return err
	}
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		return fmt.Errorf("informer cache sync")
	}
	go c.worker(ctx)
	<-ctx.Done()
	c.queue.ShutDown()
	return nil
}

type controller struct {
	dyn   dynamic.Interface
	rec   *Reconciler
	queue workqueue.TypedRateLimitingInterface[string]
	log   *slog.Logger
}

func (c *controller) enqueue(obj any) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		tomb, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		u, ok = tomb.Obj.(*unstructured.Unstructured)
		if !ok {
			return
		}
	}
	c.queue.Add(u.GetNamespace() + "/" + u.GetName())
}

func (c *controller) worker(ctx context.Context) {
	for {
		key, shutdown := c.queue.Get()
		if shutdown {
			return
		}
		err := c.sync(ctx, key)
		if err != nil {
			c.log.Error("reconcile", "key", key, "err", err)
			c.queue.AddRateLimited(key)
		} else {
			c.queue.Forget(key)
			c.queue.AddAfter(key, requeue)
		}
		c.queue.Done(key)
	}
}

func (c *controller) sync(ctx context.Context, key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	syncCtx, cancel := context.WithTimeout(ctx, syncBudget)
	defer cancel()
	u, err := c.dyn.Resource(GVR).Namespace(ns).Get(syncCtx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	cl, err := ClusterFrom(u)
	if err != nil {
		return err
	}
	c.log.Info("reconcile", "namespace", cl.Namespace, "name", cl.Name, "generation", cl.Generation)
	return c.rec.Reconcile(syncCtx, cl)
}
