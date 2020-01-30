# Datadog

You can add operational visibility to your Convox environments with Datadog.

## Sign up for Datadog

If you don’t have an account already, [sign up for Datadog](https://app.datadoghq.com/signup). You’ll need an API key that lets you send data from Convox to the Datadog service.

## Deploy the Datadog Agent

You can deploy the datadog agent as a Convox app with a very simple `convox.yml` manifest:

```
services:
  datadog:
    agent: true
    ports:
      - 8125/udp
      - 8126/tcp
    image: datadog/agent:latest
    environment:
      - DD_API_KEY
      - DD_APM_ENABLED=true
    privileged: true
    scale:
      cpu: 128
      memory: 128
    volumes:
      - /sys/fs/cgroup/:/host/sys/fs/cgroup/
      - /proc/:/host/proc/
      - /var/run/docker.sock:/var/run/docker.sock
```

### Application Metrics

To forward application metrics to Datadog you'll need the host IP address. You can get it with:

    $ ip route list match 0/0 | awk '{print $3}'

## Logging Endpoint

To integrate Datadog as a logging endpoint with our [Syslog](/deployment/syslogs) resource:

  * Go to [Syslog-Ng Integration](https://docs.datadoghq.com/integrations/syslog_ng/?tab=datadogussite) to check the forwarding destination.  This currently differs between the US site (`intake.logs.datadoghq.com:10516`) and the EU site (`tcp-intake.logs.datadoghq.eu:443`)
  * Suggested `Format="INSERT-YOUR-API-KEY-HERE <22>1 {DATE} {GROUP} {SERVICE} {CONTAINER} - [metas ddsource=\"{GROUP}\" ddtags=\"container_id:{CONTAINER}\"] {MESSAGE}"` where you replace `INSERT-YOUR-API-KEY-HERE` with your Datadog API key 😉

For example:

    $ convox rack resources create syslog Format="123457890abcdef1234567890 <22>1 {DATE} {GROUP} {SERVICE} {CONTAINER} - - [metas ddsource=\"{GROUP}\" ddtags=\"container_id:{CONTAINER}\"] {MESSAGE}" Url=tcp+tls://intake.logs.datadoghq.com:10516

Link the created Syslog resource to your app:

    $ convox rack resources link syslog-3785 --app example