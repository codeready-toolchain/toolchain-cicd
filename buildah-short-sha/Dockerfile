FROM quay.io/buildah/stable:v1.11.0

LABEL maintainer "Devtools <devtools@redhat.com>"
LABEL author "Devtools <devtools@redhat.com>"

ENV BUILDAH=/usr/local/bin/buildah-short-sha \
    USER_UID=1001 \
    USER_NAME=toolchain-cd \
    LANG=en_US.utf8

# install script locally
COPY buildah-short-sha/buildah-short-sha ${BUILDAH}



