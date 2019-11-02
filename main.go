package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
)

const defaultServingPort = ":80"
const token_identifier = "__UPDATER_TOKEN"

var registry string
var credentials map[string][]string

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
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	serviceName := r.URL.Query().Get("serviceName")
	imageName := r.URL.Query().Get("imageName")
	imageVersion := r.URL.Query().Get("imageVersion")
	token := r.URL.Query().Get("token")
	if serviceName == "" || imageVersion == "" || imageName == "" || token == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	args := filters.NewArgs()
	args.Add("name", serviceName)

	tokens, ok := credentials[serviceName]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	success := false
	for _, t := range tokens {
		if t == token {
			success = true
		}
	}
	if !success {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	services, err := cli.ServiceList(context.Background(), types.ServiceListOptions{Filters: args})
	if err != nil {
		panic(err)
	}
	if len(services) > 1 {
		panic("Something goes wrong")
	}

	for _, service := range services {
		// registry = docker.pkg.github.com/lbejiuk/private_pkg/
		// imageName = themarkz_back
		// imageVersion = $APP_NAME
		contSpec := &service.Spec.TaskTemplate.ContainerSpec
		imageNameChecker := regexp.MustCompile("^" + registry + imageName)
		if !imageNameChecker.MatchString(contSpec.Image) {
			// try to change image
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		newImage := fmt.Sprintf("%s%s:%s", registry, imageName, imageVersion)
		contSpec.Image = newImage
		resp, err := cli.ServiceUpdate(context.Background(), service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			panic(err)
		}
		fmt.Println(resp)
	}
}
