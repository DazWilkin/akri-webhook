ARG PROJECT="webhook"

FROM golang:1.15 as build

ARG PROJECT
WORKDIR /${PROJECT}

ARG MODULE="github.com/deislabs/akri/${PROJECT}"

COPY go.mod .
RUN go mod download

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -installsuffix cgo \
    -o /bin/${PROJECT} \
    ${MODULE}/cmd/client


FROM gcr.io/distroless/base-debian10

ARG PROJECT

COPY --from=build /bin/${PROJECT} /server

ENTRYPOINT ["/server"]
CMD ["--tls-crt-file=/path/to/crt","--tls-key-file=/path/to/key","--port=8443"]
