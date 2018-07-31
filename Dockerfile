#
# build filesystem
#
FROM alpine:latest as filesystem_build
RUN \
        echo -e "\n# preparing filesystem" && \
        mkdir -p /tmp/etc/ && \
        echo "root:x:0:0:root:/:/shell-doesnt-exist" > /tmp/etc/passwd && \
        echo "nobody:x:99:99:Nobody:/:/shell-doesnt-exist" >> /tmp/etc/passwd && \
        echo "root:x:0:root" > /tmp/etc/group && \
        echo "nobody:x:99:nobody" >> /tmp/etc/group && \
        chmod -R 0444 /tmp/etc


#
# build ldap_proxy
#
FROM golang:latest as proxy_build
WORKDIR /go

COPY src/ldap_proxy.go ./ldap_proxy.go

RUN \
       echo "\n# downloading dependencies" && \
       go get -u github.com/nmcclain/ldap && \
       go get -u github.com/go-ldap/ldap && \
       echo "\n# formatting software" && \
       go fmt . && \
       echo "\n# testing software" && \
       go test -v && \
       echo "\n# build software" && \
       CGO_ENABLED=0 GOOS=linux go build -a -tags "netgo static_build" -installsuffix netgo -ldflags "-w -s" -o ldap_proxy ldap_proxy.go 


#
# build ldap_server
#
FROM golang:latest as server_build
WORKDIR /go

COPY src/ldap_server.go ./ldap_server.go

RUN \
       echo "\n# downloading dependencies" && \
       go get -u github.com/nmcclain/ldap && \
       echo "\n# formatting software" && \
       go fmt . && \
       echo "\n# testing software" && \
       go test -v && \
       echo "\n# build software" && \
       CGO_ENABLED=0 GOOS=linux go build -a -tags "netgo static_build" -installsuffix netgo -ldflags "-w -s" -o ldap_server ldap_server.go 


#
# build ldap_search
#
FROM golang:latest as search_build
WORKDIR /go

COPY src/ldap_search.go ./ldap_search.go

RUN \
       echo "\n# downloading dependencies" && \
       go get -u github.com/nmcclain/ldap && \
       echo "\n# formatting software" && \
       go fmt . && \
#       echo "\n# testing software" && \
#       go test -v && \
       echo "\n# build software" && \
       CGO_ENABLED=0 GOOS=linux go build -a -tags "netgo static_build" -installsuffix netgo -ldflags "-w -s" -o ldap_search ldap_search.go 


#
# test software binaries
#
FROM alpine:latest
COPY --from=server_build /go/ldap_server /ldap_server
COPY --from=search_build /go/ldap_search /ldap_search
COPY src/test.sh /test.sh
RUN /test.sh


#
# target image
#
FROM scratch
LABEL name="ldp-ldap-proxy-go"
LABEL description="GITC IAM - ldp-ldap-proxy-go - docker image"
LABEL maintainer="GITC IAM"
WORKDIR /
COPY --from=proxy_build /go/ldap_proxy /ldap_proxy
COPY --from=server_build /go/ldap_server /ldap_server
COPY --from=filesystem_build /tmp/etc /etc

USER nobody

ENTRYPOINT ["/ldap_proxy"]
