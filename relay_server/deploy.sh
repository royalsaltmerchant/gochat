#!/bin/bash

rsync -av --progress -e "ssh -i ~/.ssh/id_rsa" \
  auth.go config.go dispatch.go email.go events.go host.go main.go notifications.go sessions.go socket.go turn.go types.go user.go call_auth.go stripe.go \
  relay-migrations static \
  root@64.23.134.139:/root/relay_server
