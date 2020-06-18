#!/bin/bash

if [ "$EUID" -ne 0 ]
  then echo "Please run as root"
  exit 1
fi

set -eux

SNAP=/snap/microk8s/current/
SNAP_DATA=/var/snap/microk8s/current/

microk8s.stop

microk8s.enable ha-cluster
microk8s.enable dashboard

cp "$SNAP_DATA"/args/kube-apiserver "$SNAP_DATA"/args/kube-apiserver.backup
cat <<EOT > "$SNAP_DATA"/args/kube-apiserver
--cert-dir=\${SNAP_DATA}/certs
--service-cluster-ip-range=10.152.183.0/24
--authorization-mode=RBAC,Node
--service-account-key-file=\${SNAP_DATA}/certs/serviceaccount.key
--client-ca-file=\${SNAP_DATA}/certs/ca.crt
--tls-cert-file=\${SNAP_DATA}/certs/server.crt
--tls-private-key-file=\${SNAP_DATA}/certs/server.key
--kubelet-client-certificate=\${SNAP_DATA}/certs/server.crt
--kubelet-client-key=\${SNAP_DATA}/certs/server.key
--secure-port=16443
--token-auth-file=\${SNAP_DATA}/credentials/known_tokens.csv
--storage-dir \${SNAP_DATA}/var/kubernetes/backend/
--storage-backend=dqlite
--insecure-port=0

# Enable the aggregation layer
--requestheader-client-ca-file=\${SNAP_DATA}/certs/front-proxy-ca.crt
--requestheader-allowed-names=front-proxy-client
--requestheader-extra-headers-prefix=X-Remote-Extra-
--requestheader-group-headers=X-Remote-Group
--requestheader-username-headers=X-Remote-User
--proxy-client-cert-file=\${SNAP_DATA}/certs/front-proxy-client.crt
--proxy-client-key-file=\${SNAP_DATA}/certs/front-proxy-client.key
#~Enable the aggregation layer
--allow-privileged=true
EOT

systemctl start snap.microk8s.daemon-apiserver

# TODO do some proper wait here
sleep 10

rm -rf db
./migrator --mode backup-dqlite --endpoint "unix:///var/snap/microk8s/current/var/kubernetes/backend/kine.sock" --db-dir db --debug

microk8s.disable ha-cluster


cp "$SNAP_DATA"/args/etcd "$SNAP_DATA"/args/etcd.backup
cat <<EOT > "$SNAP_DATA"/args/etcd
--data-dir=\${SNAP_COMMON}/var/run/etcd
--advertise-client-urls=http://127.0.0.1:12379
--listen-client-urls=http://0.0.0.0:12379
--enable-v2=true
EOT
systemctl restart snap.microk8s.daemon-etcd
sleep 20
./migrator --mode restore-to-etcd --endpoint "http://127.0.0.1:12379" --db-dir db --debug


cp "$SNAP_DATA"/args/kube-apiserver.backup "$SNAP_DATA"/args/kube-apiserver
cp "$SNAP_DATA"/args/etcd.backup "$SNAP_DATA"/args/etcd

sleep 10
microk8s.start
