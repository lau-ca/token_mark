#!/usr/bin bash

cd /root/gateway/master
docker compose down -v

cd /root/gateway/work
docker compose down -v

docker rmi calciumion/new-api:latest

cd /root/gateway/
docker load -i new-api-latest.tar

cd /root/gateway/master
docker compose up -d

cd /root/gateway/work
docker compose up -d