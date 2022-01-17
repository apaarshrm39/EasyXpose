package main

import (
	"context"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	network "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type xposer struct {
	clientset *kubernetes.Clientset
	depLister v1.DeploymentLister
	hasSynced cache.InformerSynced
	queue     workqueue.RateLimitingInterface
}

func new(clientset kubernetes.Clientset, infofac informers.SharedInformerFactory) *xposer {
	x := &xposer{
		clientset: &clientset,
		depLister: infofac.Apps().V1().Deployments().Lister(),
		hasSynced: infofac.Apps().V1().Deployments().Informer().HasSynced,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "xposeq"),
	}
	infofac.Apps().V1().Deployments().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    x.handleAdd,
		DeleteFunc: x.handleDelete,
		UpdateFunc: x.handleUpdate,
	})
	return x
}

func (x xposer) run(ch <-chan struct{}) {
	klog.Infoln("Starting the xposer controller")
	// Make sure informer cache is synced succesfully
	// we need to pass it a channel of struct{}
	// if this is not done then something went wrong
	if !cache.WaitForCacheSync(ch, x.hasSynced) {
		klog.Errorln("Cache could not sync due to some reason")
	}

	go wait.Until(x.worker, 1*time.Second, ch)

	<-ch
}

func (x xposer) worker() {

	for x.process() {

	}

}

func (x xposer) process() bool {
	// Getting item from queue
	item, shutdown := x.queue.Get()
	if shutdown {
		return false
	}

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorln("error while retrieving key from cache", err)
		return false
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorln("error while splitting the key", err)
		return false
	}

	err = x.exposeServie(ns, name)
	err = x.exposeIngress(ns, name)
	if err != nil {
		return false
	}

	x.queue.Done(item)

	return true
}

func (x xposer) exposeServie(ns, n string) error {
	deploy, err := x.depLister.Deployments(ns).Get(n)
	if err != nil {
		klog.Errorln("error: ", err)
		return err
	}

	labels := deploy.Labels

	if val, ok := labels["apaarshrm/port"]; ok {
		//do something here
		//}
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n + "-svc",
				Namespace: ns,
			},
			Spec: corev1.ServiceSpec{
				Selector: deploy.Labels,
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Port: stringToInt32(val),
					},
				},
			},
		}

		_, err = x.clientset.CoreV1().Services(ns).Create(context.TODO(), &svc, metav1.CreateOptions{})
		if err != nil {
			klog.Errorln(err)
			return err
		}
		klog.Info("Service created succesfully for ", n)
		return nil
	} else {
		klog.Infoln("The deployment does not have the required label", n)
		return nil
	}
}

func (x xposer) exposeIngress(ns, n string) error {
	deploy, err := x.depLister.Deployments(ns).Get(n)
	if err != nil {
		klog.Errorln("error: ", err)
		return err
	}

	anno := deploy.Annotations

	labels := deploy.Labels
	val, foundV := anno["apaarshrm/host"]
	path, foundP := anno["apaarshrm/path"]
	port, foundPort := labels["apaarshrm/port"]

	if foundV && foundP && foundPort {
		pathType := "Prefix"
		//do something here
		//}
		ing := network.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n + "-ingress",
				Namespace: ns,
			},
			Spec: network.IngressSpec{
				Rules: []network.IngressRule{
					network.IngressRule{
						Host: val,
						IngressRuleValue: network.IngressRuleValue{
							HTTP: &network.HTTPIngressRuleValue{
								Paths: []network.HTTPIngressPath{
									network.HTTPIngressPath{
										Path:     path,
										PathType: (*network.PathType)(&pathType),
										Backend: network.IngressBackend{
											Service: &network.IngressServiceBackend{
												Name: n + "-svc",
												Port: network.ServiceBackendPort{
													Number: stringToInt32(port),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		_, err = x.clientset.NetworkingV1().Ingresses(ns).Create(context.TODO(), &ing, metav1.CreateOptions{})
		//_, err = x.clientset.CoreV1().Services(ns).Create(context.TODO(), &svc, metav1.CreateOptions{})
		if err != nil {
			klog.Errorln(err)
			return err
		}
		klog.Info("Ingress created succesfully for ", n)
		return nil
	} else {
		klog.Infoln("The deployment does not have the required labels for Ingress", n)
		return nil
	}
}

func (x xposer) handleAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorln(err)
	}
	klog.Infoln("deplyment was added", key)
	x.queue.Add(obj)
}

func (x xposer) handleDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorln(err)
	}
	klog.Infoln("deplyment was Deleted", key)
}

func (x xposer) handleUpdate(oldObj interface{}, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		klog.Errorln(err)
	}
	klog.Infoln("deplyment was Updated", key)
	x.queue.Add(newObj)
}

func stringToInt32(s string) int32 {
	num, err := strconv.ParseInt(s, 0, 32)
	if err != nil {
		klog.Errorln(err)
	}
	return int32(num)
}
