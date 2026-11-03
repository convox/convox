FROM httpd:2.4-trixie@sha256:c6b7745aa371570cc8460a9f08c39989583d4aa7bc8e3ed8bd77a492803ea704

RUN apt-get -qq update && apt-get -qq install -y curl postgresql-client redis ncat mariadb-client

COPY ./scripts/ /usr/scripts/

RUN find /usr/scripts/ -type f -iname "*.sh" -exec chmod +x {} \;

COPY entry.sh entry.sh

RUN chmod +x ./entry.sh

ENTRYPOINT [ "sh", "./entry.sh" ]

CMD [ "httpd-foreground" ]
