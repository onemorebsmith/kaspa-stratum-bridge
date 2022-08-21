FROM golang:1.18 as builder


WORKDIR /go/src/app
ADD go.mod .
ADD go.sum .
RUN go mod download

ADD . .
RUN go build -o /go/bin/app ./cmd/bridge


FROM gcr.io/distroless/base:nonroot
COPY --from=builder /go/bin/app /
COPY cmd/bridge/config.yaml /

WORKDIR /
ENTRYPOINT ["/app"]
