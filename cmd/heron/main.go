package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/kballard/go-shellquote"
)

var Auth = `{"470778123668.dkr.ecr.us-east-1.amazonaws.com/dev11/nodejs":{"Username":"AWS","Password":"eyJwYXlsb2FkIjoiR1R6R3B0VmFSQVhSdTRwY3ZpM0xQQXNJaXdmc21qOXZ2REMzZ1NKT2YvekdWYWJQZVh3ZTVVWDMzUzZLZzl0eHNFUWFWek5EOEVLY0s0MFowT0hSdXhsblNEQkc2Wk1xa3VPZmV3Q1lTY2tYTUc1NkpaazRVT0h3QWxsZnVQNEsrWHEvTWhRZThXYzBtTlFJUURkOHp5Z0RlREZCUHRMZXBOVzdIQWpHRVdLWTJjVzMzZzdBMHp5TXozTDJHcE8zeFlvWWtvY0V0Nmh6MVRVRUd2SEFvZW4vOXZSdHUybU4zUitpcm9scGYvNEYzV1JnQlZZM0dHbXBOTkVmOWVSZGR2WDl0OHJseDUzTVVKaWVvanhOTS9MMXlkK2pMTC8waUhUaVFuM3RzeGZuc21SOFp2QWNvOFEvSXhFankwT1JmYkd5VkpyYk5qZ1psdHQ5V3RtbFpkbk04bmFrT3ZvOVlzcjRibGVjdE41TGlDV1RIaDBUNCtLWVA1YTl3bEVsYWszcXdGN0U2bWM3ZkV4enRNZHRYcGxRMjhObEFQdjlSYkJyU3Z0Qzh3ZHk0VWdoYWlHSW9aZ21RNHNkVmtSMllnMC9aZ215ay9xYU1XcUhvQTFIaUhocnJyRHZWQ0tMSTdhMU1XZFh6bkdwMnozTG44eXd5eWtMbnp1cVl3RXllZ0xWSFJUS1AvME5UMmVUSGhIVUdLZndSMEs2T2ZBdk9qNUNaMmwwcUV3K2J5eUxSYWs4dnFvRmx0ejFRNGxhTm5wN1dKVUtKWUJYS1QvTXFTY0F1cWZtUDVZaksyVkNlNW9GQXBCSG9zL1Y2U201SXB3RkZHZnQxeHhRMnNub0w1TW9pb1gwSUR2dUVpR3MwZm1oQVFoR2d5TTdUMnZVcDN4WklOVjRDV0Y0bXB5cVdhS0R4NVhtZDhHZHp6aU5VaVphNjU1d3g4VENPeGJEL1B4WXQyc1pIbmx2b05LdVVoSXAxWHpJK1lTSkE2RndJNVdWeWJISzZTNXFOOHJqdERuOHFFa2k5citZeGg5SWJTR2IzeDUraGtmUGR3amwvY2taZHdyYkVsK3JGbzEzcjBONnVkdEsyL2ZaNi8rbi9pVmFadHZmUi93ajJsUDNrL0pBNng5U0lNOFp0VlNxVHdZc0V4QXpRSzd0MnUrSWNQMHlZRjJ0WXZIa1BkRWNYcTdrSUFPUUxWWHJsMW5Bc3FhMzZFSGpaZXR6NER6TXJCNXJPd2g5KzdBM09DLzVDelJGVTZVWElzbGZyT0VIaVRrOXVyM3daM3RNeHNpeG96cTZ2TVFDQWZWdUFmcW5QR2I0YlFyMUxMQWV2MW9oVFdjQ1RHNG1qa0pwaHNibzNldnNVT1B0M2U0eW1wZ21mSUw2U0tTUjV0aXZ0Y3p2ZCs2S2VSUCtGR2dSNmVaSFpVZks0a3JmQ0l6Z0JaZWxKczRERnpQY3hMWnVybE41NjVGTnhsN1FIcWdsMzE4RmtjNHhBeHlJUTNFbjZhd2FpWUVtcUJQSzQ5cWtpYTdkTHowUWMxbmVIQW5yNVB0QVhNdnU1OVFIL2ZzKzlCbC9LZi94ZlowZTZsalJwcUkvNUxsRlVidytuaFNpaWJva0NSajRoTnM2QVNXZzhRWjE0WGFqMEFydjBrZDhLd25vYlYxZGlRcitPVlZYV2JiM2RqYmlJQTFVd24xK0VDaEZMdWhPazNqUTlReEF4bHdJZVljNTZZNkhrMlhHdkxFOEh5VGNsWFpTNi8xc3FRTkFYTjBLUEZ1ZWRyODBGeVlrUTlRWnJYcWI2ZGNKT3dzaUt0ZXpxV2hnQnJ5K2g4UnZ2MDF2di9lV1RjSWNiQlBDeU5HcTdCd0V0Yk9RRVZ2NlZ1WWxodkFNMTRvSEdiV0t1U3NCUzVCZTNma01YdzhmNVNEOUJITHB6a0lFVnV1TE8wZ256T0tSS2ZFQUxKR2RlTmRPRk9ZdllwejlHK3pMT0p0WmVleUdJa0c5STl6QVhLekl3R1ZkWUFmVUZnUERETEg0N0lDckFmb3N5R2dvQ0hPTXJhT1prL0E2aFBacEkwYnR1c1hDTjZBdDhnWDRIK2tQUHNxT3h4NmJnUHlURjViSjlKSk9udkdSeFFNZE42dWx1WmVudkwyd2JobEpWRVczODh3OUMxUTkyUUhncmh3d3JmT0MzRzlvOFIrTGxMZ3dUdDZCTTBSUmxUY0RCejc1Rkpjam5KdGFSWTJZVFh2RlpzQ0QwTUc2a09seEJmaHZ4cnV5d2VSY1h6N0hRc3VhMGxnVzkzMHBMSHpCeTUyWVJhUWR0LzRtQ2UraG1mam9JYWtPb05SU0FHRnNOZkNpdW9aNHJyWVJ4R09sZmZjc1JWVFV5ckd2QTVMWUlRZGhyWVRTeU15NzRnRDZ3VWhEb1RNeVYvUHBvWXJSUitUb0JBdmMzbjZMa1dXNnZ2ZEplcXYzVmt1MiIsImRhdGFrZXkiOiJBUUVCQUhod20wWWFJU0plUnRKbTVuMUc2dXFlZWtYdW9YWFBlNVVGY2U5UnE4LzE0d0FBQUg0d2ZBWUpLb1pJaHZjTkFRY0dvRzh3YlFJQkFEQm9CZ2txaGtpRzl3MEJCd0V3SGdZSllJWklBV1VEQkFFdU1CRUVERkJwWUkwdHZEWXNBZWlObFFJQkVJQTdHZG12VTJUUXJmWUNSY0VOamZyNWtxUEZ0cXpiV05WZVlBMUxWQ0dubGhrSE1sOTB0Q1FmQzV0bFdaUVVkWCtxYnQ1ek5ETy8yVjREcnVNPSIsInZlcnNpb24iOiIyIiwidHlwZSI6IkRBVEFfS0VZIiwiZXhwaXJhdGlvbiI6MTY1OTQyNDQxOX0="}}`

// 470778123668.dkr.ecr.us-east-1.amazonaws.com/dev11/nodejs
func login() error {
	var registries map[string]struct {
		Username string
		Password string
	}

	type auth struct {
		Auth string `json:"auth"`
	}

	type authConfig struct {
		Auths map[string]auth
	}

	if err := json.Unmarshal([]byte(Auth), &registries); err != nil {
		return err
	}

	ac := authConfig{Auths: make(map[string]auth)}
	for host, entry := range registries {
		ac.Auths[host] = auth{
			Auth: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", entry.Username, entry.Password))),
		}
	}

	f, err := json.Marshal(ac)
	if err != nil {
		return err
	}

	// home := os.Getenv("HOME")
	os.WriteFile(fmt.Sprintf("config.json"), f, 0755)

	return nil
}

func entrypoint() {
	// fd, _ := os.Open("Dockerfile")
	// defer fd.Close()

	d, _ := ioutil.ReadFile("Dockerfile")

	// println(">read", string(d))

	r := regexp.MustCompile(`(?i)entrypoint \[(?P<arr>.*)\]|entrypoint (?P<str>".*")`)
	groups := map[string]string{}
	gnames := r.SubexpNames()

	for ix, xp := range r.FindStringSubmatch(string(d)) {
		groups[gnames[ix]] = xp
	}

	if groups["str"] != "" {
		println(">>> string")
		cmd := groups["str"]
		cmd = strings.ReplaceAll(cmd, "\"", "")
		args := shellquote.Join(strings.Split(cmd, " ")...)
		fmt.Println(args)
		// fmt.Println(shellquote.Split(args))

		return
	}

	if groups["arr"] != "" {
		e := groups["arr"]
		e = strings.ReplaceAll(e, "\"", "")
		// println(e)
		args := shellquote.Join(strings.Split(e, ",")...)
		fmt.Println(args)
		// fmt.Println(shellquote.Split(args))
	}
}

func main() {
	// entrypoint()
	// fmt.Println(strings.Split("470778123668.dkr.ecr.us-east-1.amazonaws.com/dev11/console:web.BINTIFCAPJT", "/"))
	// data, err := ioutil.ReadFile(fmt.Sprintf("/home/heron/projects/convox/provider/k8s/testdata/release-%s.yml", "basic-app"))
	// fmt.Println(err)
	// println(string(data))
	cmd := exec.Command("skopeo", "inspect", "--config", "docker://heronrs/test:1")
	data, err := cmd.CombinedOutput()
	inspect := struct {
		Config struct {
			Entrypoint []string
		}
	}{}

	println("skopeo output", string(data))
	if err != nil {
		println("failed to retrieve image entrypoint", "-", err.Error(), string(data))
	}

	err = json.Unmarshal(data, &inspect)
	if err != nil {
		println("failed to retrieve image entrypoint", "-", err.Error(), string(data))
	}

	var rr []string
	println(inspect.Config.Entrypoint)
	println(len(rr))
}
