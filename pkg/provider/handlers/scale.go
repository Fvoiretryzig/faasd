package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"

	"github.com/openfaas/faas-provider/proxy"
	"github.com/openfaas/faas-provider/types"
	"github.com/openfaas/faasd/vendor/github.com/openfaas/faas-provider/httputil"
)

func MakeReplicaUpdateHandler(client *containerd.Client, cni gocni.CNI, config types.FaaSConfig, resolver proxy.BaseURLResolver) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		bodyBytes, _ := ioutil.ReadAll(r.Body)
		log.Printf("[Scale] request: %s\n", string(bodyBytes))
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		//defer r.Body.Close()

		req := types.ScaleServiceRequest{}
		err := json.Unmarshal(bodyBytes, &req)
		if err != nil {
			log.Printf("[Scale] error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		namespace := getRequestNamespace(readNamespaceFromQuery(r))

		// Check if namespace exists, and it has the openfaas label
		valid, err := validNamespace(client.NamespaceService(), namespace)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !valid {
			http.Error(w, "namespace not valid", http.StatusBadRequest)
			return
		}

		name := req.ServiceName
		proxyClient := NewProxyClientFromConfig(config)
		tmpAddr, resolveErr := resolver.Resolve(name)
		if resolveErr != nil {
			// TODO: Should record the 404/not found error in Prometheus.
			//log.Printf("resolver error: no endpoints for %s: %s\n", functionName, resolveErr.Error())
			httputil.Errorf(w, http.StatusServiceUnavailable, "No endpoints available for: %s.", name)
			return
		}
		addrStr := tmpAddr.String()
		addrStr += "/scale-updater"
		functionAddr, _ := url.Parse(addrStr)
		proxyReq, err := buildProxyRequest(r, *functionAddr) //no params for replicaUpdater handler
		if err != nil {
			httputil.Errorf(w, http.StatusInternalServerError, "Failed to resolve service: %s.", name)
			return
		}
		if proxyReq.Body != nil {
			defer proxyReq.Body.Close()
		}
		reqctx := r.Context()
		response, err := proxyClient.Do(proxyReq.WithContext(reqctx)) //send request to watchdog
		if err != nil {
			log.Printf("[Scale] error with proxy request to: %s, %s\n", proxyReq.URL.String(), err.Error())
			httputil.Errorf(w, http.StatusInternalServerError, "Can't reach service for: %s.", name)
			return
		}
		if response.Body != nil {
			defer response.Body.Close()
		}
		log.Printf("[Scale] Set replicas - %s, %d\n", name, req.Replicas)
		w.WriteHeader(response.StatusCode)

		if _, err := GetFunction(client, name, namespace); err != nil {
			msg := fmt.Sprintf("service %s not found", name)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		/*ctx := namespaces.WithNamespace(context.Background(), namespace)

		ctr, ctrErr := client.LoadContainer(ctx, name)
		if ctrErr != nil {
			msg := fmt.Sprintf("cannot load service %s, error: %s", name, ctrErr)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		var taskExists bool
		var taskStatus *containerd.Status

		task, taskErr := ctr.Task(ctx, nil)
		if taskErr != nil {
			msg := fmt.Sprintf("cannot load task for service %s, error: %s", name, taskErr)
			log.Printf("[Scale] %s\n", msg)
			taskExists = false
		} else {
			taskExists = true
			status, statusErr := task.Status(ctx)
			if statusErr != nil {
				msg := fmt.Sprintf("cannot load task status for %s, error: %s", name, statusErr)
				log.Printf("[Scale] %s\n", msg)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			} else {
				taskStatus = &status
			}
		}

		createNewTask := false

		// Scale to zero
		if req.Replicas == 0 {
			// If a task is running, pause it
			if taskExists && taskStatus.Status == containerd.Running {
				if pauseErr := task.Pause(ctx); pauseErr != nil {
					wrappedPauseErr := fmt.Errorf("error pausing task %s, error: %s", name, pauseErr)
					log.Printf("[Scale] %s\n", wrappedPauseErr.Error())
					http.Error(w, wrappedPauseErr.Error(), http.StatusNotFound)
					return
				}
			}
		}

		if taskExists {
			if taskStatus != nil {
				if taskStatus.Status == containerd.Paused {
					if resumeErr := task.Resume(ctx); resumeErr != nil {
						log.Printf("[Scale] error resuming task %s, error: %s\n", name, resumeErr)
						http.Error(w, resumeErr.Error(), http.StatusBadRequest)
						return
					}
				} else if taskStatus.Status == containerd.Stopped {
					// Stopped tasks cannot be restarted, must be removed, and created again
					if _, delErr := task.Delete(ctx); delErr != nil {
						log.Printf("[Scale] error deleting stopped task %s, error: %s\n", name, delErr)
						http.Error(w, delErr.Error(), http.StatusBadRequest)
						return
					}
					createNewTask = true
				}
			}
		} else {
			createNewTask = true
		}

		if createNewTask {
			deployErr := createTask(ctx, ctr, cni)
			if deployErr != nil {
				log.Printf("[Scale] error deploying %s, error: %s\n", name, deployErr)
				http.Error(w, deployErr.Error(), http.StatusBadRequest)
				return
			}
		}*/
	}
}
