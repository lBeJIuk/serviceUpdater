```yaml
version: "3.7"
  services:
    web: 
      image: nginx:latest

    updater:
      image: serviceUpdater
      ports:
        - "8080:9000"
      environment:
        - web__UPDATER_TOKEN=sercret_token_
        - SERVING_PORT=":9000"
        - IMAGE_REGISTRY="docker.pkg.github.com/UserName/repo/"
```

```shell script
http://myServer:8080/service/update?service_name=web&imageName=nginx&imageVersion=alpine&token=sercret_token_
```