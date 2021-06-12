package main

import (
	"context"
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
	log.Println(r.URL.Query())
	if serviceName == "" || imageVersion == "" || imageName == "" || token == "" {
		log.Println("serviceName/imageName/imageVersion/token are empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tokens, ok := credentials[serviceName]
	if !ok {
		log.Println("no credentials for " + serviceName)
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
		log.Println("Wrong token for " + serviceName)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	args := filters.NewArgs()
	args.Add("name", serviceName)
	services, err := cli.ServiceList(context.Background(), types.ServiceListOptions{Filters: args})
	if err != nil {
		panic(err)
	}

	if len(services) != 1 {
		log.Println("Something went wrong")
		log.Println("Count:", len(services))
		log.Println("serviceName did not found", serviceName)
		w.WriteHeader(http.StatusBadRequest)
	}

	for _, service := range services {
		// registry = docker.pkg.github.com/lbejiuk/private_pkg/
		// imageName = themarkz_back
		// imageVersion = $APP_NAME
		contSpec := &service.Spec.TaskTemplate.ContainerSpec
		imageNameChecker := regexp.MustCompile("^" + registry + imageName)
		if !imageNameChecker.MatchString(contSpec.Image) {
			log.Println("Try to use another image for " + serviceName)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		newImage := fmt.Sprintf("%s%s:%s", registry, imageName, imageVersion)
		contSpec.Image = newImage
		log.Println("Trying to update", service.ID, service.Version)
		resp, err := cli.ServiceUpdate(context.Background(), service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			panic(err)
		}
		log.Println(resp)
	}
}
