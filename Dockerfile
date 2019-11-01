FROM golang:latest as build
WORKDIR /tmp/build/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o builder main.go

FROM scratch
WORKDIR /var/builder/
COPY --from=build /tmp/build/builder .
ENTRYPOINT ["/var/builder/builder"]
