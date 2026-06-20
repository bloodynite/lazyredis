#!/bin/sh
set -e
if [ -f /mnt/authorized_keys ]; then
  chown testuser:testuser /home/testuser/.ssh
  chmod 700 /home/testuser/.ssh
  cp /mnt/authorized_keys /home/testuser/.ssh/authorized_keys
  chown testuser:testuser /home/testuser/.ssh/authorized_keys
  chmod 600 /home/testuser/.ssh/authorized_keys
fi
exec /usr/sbin/sshd -D -e
