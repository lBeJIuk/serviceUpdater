```yaml
version: "3.7"
  services:
    web: 
      image: nginx:latest

    updater:
      image: serviceUpdater
      volumes:
        - /var/run/docker.sock:/var/run/docker.sock
      ports:
        - "8080:9000"
      environment:
        - web__UPDATER_TOKEN=sercret_token_
        - SERVING_PORT=":9000"
        - IMAGE_REGISTRY="docker.pkg.github.com/UserName/repo/"
```

```shell script
curl -X PUT http://myServer:8080/service/update -H "_xtoken: sercret_token_" -H "Content-Type: application/json" -d '{"serviceName": "web", "imageVersion":"alpine"}'
```