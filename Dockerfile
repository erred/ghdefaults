FROM golang:alpine AS build

WORKDIR /workspace
RUN apk add --update --no-cache ca-certificates
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /bin/ghdefaults

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /bin/ghdefaults /bin/

ENTRYPOINT ["/bin/ghdefaults"]
