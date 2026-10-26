package rack

import (
	"fmt"
	"regexp"
	"strconv"
)

var nginxControllerTag = regexp.MustCompile(`(?:^|/)ingress-nginx/controller:v(\d+)\.(\d+)\.(\d+)(?:[-+@]|$)`)

var nginxPatchFloor = map[int]int{13: 9, 14: 5, 15: 1}

func (t Terraform) nginxImageWarning() string {
	vars, err := t.vars()
	if err != nil {
		return ""
	}

	return staleNginxImageWarning(vars["nginx_image"])
}

// Only the parsed version is printed, the stored value can name a private registry.
func staleNginxImageWarning(image string) string {
	m := nginxControllerTag.FindStringSubmatch(image)
	if m == nil {
		return ""
	}

	var v [3]int
	for i, s := range m[1:] {
		n, err := strconv.Atoi(s)
		if err != nil {
			return ""
		}
		v[i] = n
	}

	major, minor, patch := v[0], v[1], v[2]
	if major > 1 || (major == 1 && minor >= 13 && patch >= nginxPatchFloor[minor]) {
		return ""
	}

	return fmt.Sprintf("WARNING: nginx_image pins ingress-nginx controller v%d.%d.%d, which is older than the patched v1.13.9, v1.14.5 and v1.15.1 releases. Rack updates keep this image until nginx_image is changed or cleared with 'convox rack params set nginx_image= --force'.\n", major, minor, patch)
}
