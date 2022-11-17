FROM ubuntu:20.04

RUN sleep 900000
RUN apt update -y
RUN apt install curl git amazon-ecr-credential-helper -y

RUN curl -fSL "https://github.com/genuinetools/img/releases/download/v0.5.11/img-linux-amd64" -o "/usr/local/bin/img" && \
    chmod +x /usr/local/bin/img

RUN git clone https://github.com/convox-examples/nodejs

WORKDIR /nodejs

RUN curl -L https://github.com/moby/buildkit/releases/download/v0.10.3/buildkit-v0.10.3.linux-amd64.tar.gz -o buildctl.tar.gz && \
  tar -xvf buildctl.tar.gz && mv bin/buildctl /usr/local/bin

# RUN docker-credential-acr-env

COPY entry.sh entry.sh
RUN chmod +x entry.sh

# COPY aws.json /root/.docker/config.json

COPY az.json /root/.docker/config.json

# ENV AWS_ACCESS_KEY_ID="ASIAW3HEQCWKGJEH5KV5"
# ENV AWS_SECRET_ACCESS_KEY="KgUGq2OtJGvSgEpu/+eI/Ruuo9w6+JatvYTCD/8V"
# ENV AWS_SESSION_TOKEN="IQoJb3JpZ2luX2VjEPX//////////wEaCXVzLWVhc3QtMSJHMEUCIQD3MDXO0u1iOiA/YrA2COWCFswMsC0ttiAWQgWub0Te6AIgE5pWUN1q0qcsgMdpUoR0YwPFlvnulFJH7SsJ8awwqM0qigMIHRAAGgw0NzA3NzgxMjM2NjgiDJqf5RrlKVbrMl96MSrnAt3lImDo7yw4SHTvkqXBOgFGSLmZIZuM5jdDuguV3xxGIvhAvYfsMP+ASVvZd6Nl3Fo2k2tmag3d5w+DwTviOiCG+fQ0+Eaamgu+zA/sS4lcX3Ig8dFkOtQuSzfsm96ZEGcKywvsBJ0sxmxn6IPERuxcL01P7i3oEpfyUlyKD9gXGU0KEUCiXeotWD/Q9vjj8LJeWXmB6PaTjfLsqtc8y2Hao1XyL4Bsl8BizI8m99VPBe1W9mhYK/18M5Oy7kxuB83CBUWfN+0jLhIDqppZz+AYuzPiEDoxOacr3ynnLuaqJtGsRI2MIopHjPpN5bXB0v6LriDb5RUB/EvkFq7gMCVNZOzSwuhn2WFc3jKwEXzbScOVO0EUst3nyQOsQWP5Sfh4wBkvHLPexr4fAzVco6/P1Gou36nmCihpC9MSwBdy7I33Rd4oDnUJv0Sp4ule2prBEQbPe+LzWaS6qoz7LzU+P7PSzyG3MN3o8pUGOqYB5Sco4otOpLXU5L+iDW4nbARJgt0TdZNpu11GxFpccYlLaQ97IhXwowM3SsNH1eance4ut+pSk8Ky7XlqNphFur44NqehjF4ZaeMkSLcWrgMk5souRkld/USMVYQcaxENxmkKi4n9GNQDg1plIC8qHYMeUAq9MPwCl/pAvIpkTj0fbP23siiNMBPbEFahC44GZtbJBTPjLBXHuJ4DQqUi+UrIoh9pzA=="
# ENV AWS_REGION=us-east-1


ENTRYPOINT [ "./entry.sh" ]

