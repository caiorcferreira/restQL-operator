FROM golang:1.15.2-buster

ENV PATH "$PATH:/usr/local/kubebuilder/bin"

COPY ./install-kubebuilder.sh /usr/local/install-kubebuilder.sh
RUN chmod +x /usr/local/install-kubebuilder.sh
RUN sh /usr/local/install-kubebuilder.sh

ENTRYPOINT ["/bin/sh"]