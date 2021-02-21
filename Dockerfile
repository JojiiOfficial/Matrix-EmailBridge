FROM golang:1.16.0-alpine as builder

WORKDIR /app/matrixemailbridge

COPY ./main/*.go ./
COPY ./main/go.mod ./
COPY ./main/go.sum ./

RUN apk add --no-cache gcc musl-dev git
RUN go get -d -v 
RUN CGO_ENABLED=1
RUN go build -o main
RUN pwd && ls -lah

FROM alpine:latest

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app

COPY --from=builder /app/matrixemailbridge/main .

RUN mkdir /app/data/
RUN ls -lath

ENV BRIDGE_DATA_PATH="/app/data/"

CMD [ "/app/main"]
