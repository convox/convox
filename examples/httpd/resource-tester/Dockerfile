FROM postgres:10-alpine

RUN apk update && apk add --no-cache bash redis && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/scripts
COPY . /usr/scripts/

RUN find /usr/scripts/ -type f -iname "*.sh" -exec chmod +x {} \;

CMD ["./entrypoint.sh"]
