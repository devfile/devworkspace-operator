FROM registry.centos.org/centos/centos:latest as builder

# The following lines are needed to set the correct locale after `yum update`
# c.f. https://github.com/CentOS/sig-cloud-instance-images/issues/71
RUN localedef -i en_US -f UTF-8 C.UTF-8

ENV LANG="C.UTF-8"

RUN yum -y update && \
    yum -y install openssl gcc libc6 libc6-dev glibc-static zlib-static glibc-devel zlib-devel zlib1g-dev curl bash sudo && \
    yum clean all && \
    rm -rf /tmp/* /var/cache/yum

ADD src/main/docker/copy-libraries.sh /work/copy-libraries.sh
COPY target/*-runner /dist/application
COPY target/libsunec.so /dist/libsunec.so

RUN /work/copy-libraries.sh /dist

FROM cescoffier/native-base:latest
COPY --from=builder /dist/* /dist/
EXPOSE 9999
WORKDIR /dist
ENV LD_LIBRARY_PATH=/dist
CMD ["/dist/application", "-Dquarkus.http.host=0.0.0.0", "-Dquarkus.http.port=9999", "-Djava.runtime.name=native", "-Djava.library.path=/dist"]