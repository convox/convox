FROM alpine

RUN apk update && apk add postgresql-client bash && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/scripts
COPY . /usr/scripts/

RUN find /usr/scripts/ -type f -iname "*.sh" -exec chmod +x {} \;

CMD ["./entrypoint.sh"]
