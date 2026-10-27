FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/probelune-node ./cmd/node

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata mtr traceroute libcap \
    && addgroup -S -g 65532 probelune && adduser -S -u 65532 -G probelune probelune \
    && mkdir /data && chown 65532:65532 /data
COPY --from=build /out/probelune-node /usr/local/bin/probelune-node
RUN setcap cap_net_raw+ep /usr/local/bin/probelune-node \
    && setcap cap_net_raw+ep /usr/sbin/mtr-packet
ENV PROBELUNE_ICMP_PRIVILEGED=1
USER 65532:65532
VOLUME /data
ENTRYPOINT ["probelune-node", "--data", "/data"]
