FROM centos:7
MAINTAINER "Aslak Knutsen <aslak@redhat.com>"
ENV LANG=en_US.utf8

# Some packages might seem weird but they are required by the RVM installer.
RUN yum install epel-release -y \
    && yum --enablerepo=centosplus  --enablerepo=epel install -y \
      findutils \
      git \
      golang \
      make \
      procps-ng \
      tar \
      wget \
      which \
    && yum clean all

# Get dep for Go package management
RUN mkdir -p /tmp/go/bin
ENV GOPATH /tmp/go
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && mv /tmp/go/bin/dep /usr/bin

RUN chmod -R a+rwx ${GOPATH}

ENTRYPOINT ["/bin/bash"]
