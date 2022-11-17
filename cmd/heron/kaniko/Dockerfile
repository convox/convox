FROM gcr.io/kaniko-project/executor AS kaniko

FROM alpine:3.14.2

RUN apk --update add \
  bash \
  curl \
  git \
  jq \
  npm
# RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.21.4/bin/linux/amd64/kubectl
# RUN chmod u+x kubectl && mv kubectl /bin/kubectl

#
# Add kaniko to this image by re-using binaries and steps from official image
#
COPY --from=kaniko /kaniko/* /kaniko/
# COPY --from=kaniko /kaniko/.docker /kaniko/.docker