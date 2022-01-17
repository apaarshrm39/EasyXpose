# EasyXpose
Custom Kubernetes Controller to expose a deployment based on labels

The Controller can run outside the Cluster using the Kubeconfig context or inside the cluster as a Pod using Service Account. 

The label required to expose the pod 
```
apaarshrm/port: "80"
```

The following annotations are needed for Ingress
```
annotations:
    apaarshrm/host: ""
    apaarshrm/path: /
```