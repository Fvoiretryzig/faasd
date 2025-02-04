package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/containerd/containerd"
	"github.com/openfaas/faas-provider/types"
)

func MakeReadHandler(client *containerd.Client, config types.FaaSConfig, resolver *InvokeResolver) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		lookupNamespace := getRequestNamespace(readNamespaceFromQuery(r))
		// Check if namespace exists, and it has the openfaas label
		valid, err := validNamespace(client.NamespaceService(), lookupNamespace)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !valid {
			http.Error(w, "namespace not valid", http.StatusBadRequest)
			return
		}

		res := []types.FunctionStatus{}
		fns, err := ListFunctions(client, lookupNamespace)
		if err != nil {
			log.Printf("[Read] error listing functions. Error: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		//send request to watchdog to get replica info
		for name, fn := range fns {
			replicaInfo, err := updateReplica(name, config, resolver, r)
			if err != nil {
				log.Println("[Read] (cannot get replica count in container) read replica failed: ", err)
			}
			annotations := &fn.annotations
			labels := &fn.labels
			memory := resource.NewQuantity(fn.memoryLimit, resource.BinarySI)
			status := types.FunctionStatus{
				Name:              fn.name,
				Image:             fn.image,
				Replicas:          replicaInfo.Replicas,
				AvailableReplicas: replicaInfo.AvailableReplicas,
				InvocationCount:   replicaInfo.InvocationCount,
				Namespace:         fn.namespace,
				Labels:            labels,
				Annotations:       annotations,
				Secrets:           fn.secrets,
				EnvVars:           fn.envVars,
				EnvProcess:        fn.envProcess,
				CreatedAt:         fn.createdAt,
			}

			// Do not remove below memory check for 0
			// Memory limit should not be included in status until set explicitly
			limit := &types.FunctionResources{Memory: memory.String()}
			if limit.Memory != "0" {
				status.Limits = limit
			}

			res = append(res, status)
		}

		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}
