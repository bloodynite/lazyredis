FROM alpine:3.20

RUN apk add --no-cache openssh-server netcat-openbsd \
  && ssh-keygen -A \
  && adduser -D -h /home/testuser -s /bin/sh testuser \
  && passwd -u testuser \
  && mkdir -p /home/testuser/.ssh \
  && chmod 700 /home/testuser/.ssh

COPY sshd_config /etc/ssh/sshd_config
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22

CMD ["/entrypoint.sh"]
