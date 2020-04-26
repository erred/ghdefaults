FROM golang:alpine AS build

WORKDIR /workspace
ENV CGO_ENABLED=0
RUN apk add --update --no-cache ca-certificates
COPY . .
RUN go build -o /bin/ghdefaults

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /bin/ghdefaults /bin/

ENTRYPOINT ["/bin/ghdefaults"]
