https://graphite.readthedocs.io/en/latest/install.html

docker run -d \
 --name graphite \
 --restart=always \
 -p 8018:80 \
 -p 2003-2004:2003-2004 \
 -p 2023-2024:2023-2024 \
 -p 8125:8125/udp \
 -p 8126:8126 \
 graphiteapp/graphite-statsd
 
 docker.infini.ltd:64443/graphite-statsd:latest

root@H2-Linux:/home/medcl# docker login -u infini docker.infini.ltd:64443
root@H2-Linux:/home/medcl# docker pull docker.infini.ltd:64443/graphite-statsd:latest
latest: Pulling from graphite-statsd
Digest: sha256:a15b6a309d35b4f77d8397bd74d953dfcfa743c6aa485f6aac5d2ada0bc3a87b
Status: Image is up to date for docker.infini.ltd:64443/graphite-statsd:latest
docker.infini.ltd:64443/graphite-statsd:latest

root@H2-Linux:/home/medcl# vi ~/.docker/config.json
{
        "auths": {
                "docker.infini.ltd:64443": {
                        "auth": "aW5maW5pOmx0ZA=="
                }
        },
        "HttpHeaders": {
                "User-Agent": "Docker-Client/19.03.6 (linux)"
        }
}