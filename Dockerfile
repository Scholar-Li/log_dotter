FROM golang:1.13.15-alpine3.12 AS build
ENV  GOPATH /go
ENV APPPATH $GOPATH/src/github.com/Scholar-Li/log_dotter
WORKDIR $APPPATH
COPY . $APPPATH
RUN CGO_ENABLED=0 GOOS=linux go build -o /log_dotter

ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

EXPOSE 9094
CMD ["/log_dotter"]