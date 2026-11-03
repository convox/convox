# shellcheck shell=bash

fetch_once() {
  curl -ks --connect-timeout 5 --max-time 3 --retry 100 --retry-max-time 600 --retry-connrefused "$1"
}

fetch() {
  fetch_once "$1" && sleep 5 && fetch_once "$1"
}

# first running pod of the app's web Deployment, never a timer or `convox run` pod
web_ps() {
  convox api get "/apps/$1/processes" | jq -r '[.[] | select(.status == "running" and .name == "web" and (.id | test("^web-[a-z0-9]+-[a-z0-9]{5}$")))][0].id // empty'
}

wait_all() {
  local rc=0 p
  for p in "$@"; do
    wait "$p" || rc=1
  done
  return "$rc"
}

latest_release() {
  local tag
  tag=$(gh api repos/convox/convox/releases/latest --jq .tag_name)
  if [ -z "$tag" ] || [ "$tag" = "null" ]; then
    echo "could not resolve the latest convox release" >&2
    return 1
  fi
  echo "$tag"
}

# newest published release before $1 (empty when there is none); with a second argument, only from the same minor
previous_release() {
  local version=$1 same_minor=${2:-} tags best="" t
  tags=$(gh api --paginate 'repos/convox/convox/releases?per_page=100' --jq '.[] | select(.prerelease == false and .draft == false) | .tag_name') || return 1
  while read -r t; do
    [ "$t" = "$version" ] && break
    if [ -z "$same_minor" ] || [ "${t%.*}" = "${version%.*}" ]; then
      best=$t
    fi
  done < <({ grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' <<<"$tags"; echo "$version"; } | sort -uV)
  echo "$best"
}

# random_string ids are the generated values (the rack API password among them), and plans print env values
redact() {
  sed -u -E \
    -e 's/(random_string\.[A-Za-z0-9_]+(\[[^]]*\])?: [^[]*)\[id=.*\]$/\1[id=<redacted>]/' \
    -e '/resource "random_string"/,/^ *}$/ s/((id|result) *= *)"[^"]*"/\1"<redacted>"/g' \
    -e '/^ *[-+~]? *value *= *"/ s/"[^"]*"/"<redacted>"/g'
}

registry_ready() {
  local system=$1 host code
  [ "$(jq -r .provider <<<"$system")" = "do" ] || return 0
  host="registry.$(jq -r '."rack-domain"' <<<"$system" | sed 's/^api\.//')"
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "https://$host/v2/" || true)
  [ "$code" = "200" ] || [ "$code" = "401" ]
}

wait_ready() {
  local deadline=$((SECONDS + 1200)) system
  until system=$(convox api get /system 2>/dev/null) && [ "$(jq -r .status <<<"$system")" = "running" ] && registry_ready "$system"; do
    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "rack not ready after 20 minutes" >&2
      convox rack || true
      return 1
    fi
    sleep 15
  done
}

# a ci-YYYYMMDDHHMMSS name younger than 8 hours belongs to a run that may still be going
ci_rack_is_old() {
  local ts created
  ts=$(sed -nE 's/^ci-([0-9]{14}).*/\1/p' <<<"$1")
  [ -n "$ts" ] || return 0
  created=$(date -u -d "${ts:0:4}-${ts:4:2}-${ts:6:2} ${ts:8:2}:${ts:10:2}:${ts:12:2}" +%s 2>/dev/null) || return 0
  [ $(($(date -u +%s) - created)) -ge $((8 * 3600)) ]
}
