FROM golang as builder
WORKDIR /go/src/github.com/oliver006/drone-gcf

ADD . /go/src/github.com/oliver006/drone-gcf/
RUN GIT_COMMIT=$(git rev-list -1 HEAD) && BUILD_DATE=$(date +%F-%T) && \
    CGO_ENABLED=0 GOOS=linux go build -o drone-gcf -ldflags "-s -w -extldflags \"-static\" -X main.BuildHash=$GIT_COMMIT  -X main.BuildDate=$BUILD_DATE" .
RUN ./drone-gcf -v


FROM       google/cloud-sdk:latest as release
RUN        apt-get -y install ca-certificates
COPY       --from=builder /go/src/github.com/oliver006/drone-gcf/drone-gcf /bin/drone-gcf
ENTRYPOINT /bin/drone-gcf
