package main

import (
	"flag"
	"path/filepath"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

func main() {
	//Initialising Klogs
	klog.InitFlags(nil)
	flag.Set("v", "3")

	// Authenticating
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "kubeconfig")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "kubeconfig")
	}

	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Errorln("Could not build config %s\n", err)
		config, err = rest.InClusterConfig()
		if err != nil {
			klog.Exitf(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {

		klog.Exitf("Could Not Build Client set %s\n ", err.Error())
	}

	//fmt.Println(clientset)
	informerfactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)

	ch := make(chan struct{})
	c := new(*clientset, informerfactory)
	informerfactory.Start(ch) // start informer factory
	c.run(ch)

}
