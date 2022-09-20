FROM httpd

RUN apt update > /dev/null 2>&1
RUN apt install -y curl postgresql-client redis ncat mariadb-client > /dev/null 2>&1

COPY ./scripts/ /usr/scripts/

RUN find /usr/scripts/ -type f -iname "*.sh" -exec chmod +x {} \;
