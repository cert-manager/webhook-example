#!/bin/bash


# A really bad test powerdns setup. going to deploy a proper test pod using jenkins later

docker run -d \
  --name pdns-mysql \
  -e MYSQL_ROOT_PASSWORD=supersecret \
  -v /var/lib/mysql \
  mariadb:10.1

docker run --name pdns \
  --link pdns-mysql:mysql \
  -d \
  -p 53:53 \
  -p 53:53/udp -p 8080:8080 \
  -e MYSQL_USER=root \
  -e MYSQL_PASS=supersecret \
  -e MYSQL_PORT=3306 \
  psitrax/powerdns \
  --webserver=yes \
  --api=yes \
  --api-key=password \
  --webserver-port=8080 \
  --webserver-loglevel=detailed \
  --loglevel=10 \
  --log-dns-queries=yes \
  --master=yes \
  --disable-syslog \
  --webserver-address=0.0.0.0 \
  --webserver-allow-from=0.0.0.0/0


docker exec -it pdns pdnsutil create-zone example.com
