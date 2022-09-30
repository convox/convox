#!/bin/bash
case $provider in
   gcp)
      convox logs -a httpd --no-follow --since 10m | grep timer/example/timer-example | grep "Hello Timer"
      ;;
   *)
      convox logs -a httpd --no-follow --since 10m | grep service/web/timer-example | grep "Hello Timer"
      ;;
esac

sleep 60

numberOfPodsRunning=$(convox ps -a httpd | grep timer-concurrencyallowed | wc -l)
if [[ $(($numberOfPodsRunning)) -lt 2 ]]; then
  exit 1;
fi

numberOfPodsForbidRunning=$(convox ps -a httpd | grep timer-concurrencyforbid | wc -l)
if [[ $(($numberOfPodsForbidRunning)) -gt 1 ]]; then
  exit 1;
fi
