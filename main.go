package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/docker/docker/api/types"
)

const defaultServingPort = ":8080"
const token_identifier = "__UPDATER_TOKEN"

var registry string
var credentials map[string][]string

type requestData struct {
	ServiceName  string `json:"serviceName,omitempty"`
	ImageVersion string `json:"imageVersion,omitempty"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	servingPort := os.Getenv("SERVING_PORT")
	if servingPort == "" {
		servingPort = defaultServingPort
	}

	registry = os.Getenv("IMAGE_REGISTRY")

	credentials = map[string][]string{}
	nameChooser := regexp.MustCompile(".*" + token_identifier + "$")
	for _, element := range os.Environ() {
		variable := strings.Split(element, "=")
		variableName := variable[0]
		variableValue := variable[1]
		if nameChooser.MatchString(variableName) {
			serviceName := strings.Split(variableName, token_identifier)
			serviceTokens := strings.Split(variableValue, ",")
			credentials[serviceName[0]] = serviceTokens
		}
	}
	if len(credentials) < 1 {
		panic("Tokens are not defined")
	}

	http.HandleFunc("/service/update", serviceUpdate)
	log.Println("Start serving on port ", servingPort)
	err := http.ListenAndServe(servingPort, nil)
	if err != nil {
		panic(err)
	}
}

func serviceUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		log.Println("Unsupported method")
		errorResponse(w, "Unsupported method", http.StatusForbidden)
		return
	}

	headerContentType := r.Header.Get("Content-Type")
	if headerContentType != "application/json" {
		errorResponse(w, "Content Type is not application/json", http.StatusUnsupportedMediaType)
		return
	}

	token := r.Header.Get("_xtoken")

	var request requestData
	var unmarshalErr *json.UnmarshalTypeError
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&request)
	if err != nil {
		if errors.As(err, &unmarshalErr) {
			errorResponse(w, "Bad Request. Wrong Type provided for field "+unmarshalErr.Field, http.StatusBadRequest)
		} else {
			errorResponse(w, "Bad Request "+err.Error(), http.StatusBadRequest)
		}
		return
	}

	if request.ServiceName == "" || request.ImageVersion == "" || token == "" {
		errorResponse(w, "serviceName/imageVersion/token are empty", http.StatusBadRequest)
		return
	}

	//tokens, ok := credentials[request.ServiceName]
	//if !ok {
	//	log.Println("no credentials for " + request.ServiceName)
	//	errorResponse(w, "no credentials for "+request.ServiceName, http.StatusBadRequest)
	//	return
	//}
	//success := false
	//for _, t := range tokens {
	//	if t == token {
	//		success = true
	//	}
	//}
	//if !success {
	//	log.Println("Wrong token for " + request.ServiceName)
	//	errorResponse(w, "Wrong token for "+request.ServiceName, http.StatusBadRequest)
	//	return
	//}

	cli, err := client.NewEnvClient()
	if err != nil {
		errorResponse(w, "Docker-Cli does not start", http.StatusServiceUnavailable)
		return
	}

	args := filters.NewArgs()
	args.Add("name", request.ServiceName)
	services, err := cli.ServiceList(context.Background(), types.ServiceListOptions{Filters: args})
	if err != nil {
		errorResponse(w, "Docker-Cli does not work", http.StatusServiceUnavailable)
		return
	}

	if len(services) != 1 {
		log.Println("Something went wrong")
		log.Println("Count:", len(services))
		log.Println("serviceName did not found", request.ServiceName)
		errorResponse(w, "serviceName did not found "+request.ServiceName, http.StatusBadRequest)
		return
	}

	for _, service := range services {
		// registry = docker.pkg.github.com/lbejiuk/private_pkg/
		// request.ServiceName = themarkz_back
		// request.ImageVersion = $APP_NAME
		contSpec := &service.Spec.TaskTemplate.ContainerSpec
		imageName :=  strings.Split(contSpec.Image, ":")[0]
		newImage := fmt.Sprintf("%s%s:%s", registry, imageName, request.ImageVersion)
		contSpec.Image = newImage
		log.Println("Trying to update", service.ID, service.Version)
		resp, err := cli.ServiceUpdate(context.Background(), service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			errorResponse(w, "Docker-Cli does not work", http.StatusServiceUnavailable)
			return
		}
		log.Println(resp)
	}
}

func errorResponse(w http.ResponseWriter, message string, httpStatusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	resp := make(map[string]string)
	resp["message"] = message
	jsonResp, _ := json.Marshal(resp)
	w.Write(jsonResp)
}
