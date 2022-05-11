package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/containerd/containerd"
	"github.com/gorilla/mux"
	"github.com/openfaas/faas-provider/types"
)

func MakeReplicaReaderHandler(client *containerd.Client, config types.FaaSConfig, resolver *InvokeResolver) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		functionName := vars["name"]
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

		if f, err := GetFunction(client, functionName, lookupNamespace); err == nil {
			replicaInfo, err := updateReplica(functionName, config, resolver, r)
			if err != nil {
				log.Println("[Replica reader] replica reader error: ", err)
			}
			found := types.FunctionStatus{
				Name:              functionName,
				Image:             f.image,
				AvailableReplicas: replicaInfo.AvailableReplicas,
				Replicas:          replicaInfo.Replicas,
				InvocationCount:   replicaInfo.InvocationCount,
				Namespace:         f.namespace,
				Labels:            &f.labels,
				Annotations:       &f.annotations,
				Secrets:           f.secrets,
				EnvVars:           f.envVars,
				EnvProcess:        f.envProcess,
				CreatedAt:         f.createdAt,
			}

			functionBytes, _ := json.Marshal(found)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(functionBytes)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}
