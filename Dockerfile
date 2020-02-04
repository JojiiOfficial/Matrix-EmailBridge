FROM golang:1.13.4-buster

WORKDIR /go/src/app

COPY ./main/ .

RUN go get -d -v ./...
RUN go install -v ./...

CMD [ "app" ]