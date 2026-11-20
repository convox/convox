package cli

import (
	"slices"
	"strconv"
	"strings"
)

var rackUpdateHops = []struct{ from, below, target, via string }{
	{from: "3.24.0", below: "3.24.6", target: "3.25.13", via: "3.24.11"},
	{from: "3.0.0", below: "3.25.0", target: "3.25.13", via: "3.25.12"},
	{from: "3.0.0", below: "3.25.14", target: "3.26.0", via: "3.25.14"},
}

func rackUpdateRoute(current, target string) []string {
	cur, ok := parseRackVersion(current)
	if !ok {
		return nil
	}
	tgt, ok := parseRackVersion(target)
	if !ok {
		return nil
	}
	minor := cur[1]

	var route []string
	for _, h := range rackUpdateHops {
		if compareRackVersion(cur, h.from) >= 0 && compareRackVersion(cur, h.below) < 0 && compareRackVersion(tgt, h.target) >= 0 {
			route = append(route, h.via)
			cur, _ = parseRackVersion(h.via)
		}
	}
	if route == nil {
		return nil
	}
	route = append(route, target)

	for _, v := range route {
		next, _ := parseRackVersion(v)
		if next[1] > minor+1 {
			return nil
		}
		minor = next[1]
	}

	return route
}

func compareRackVersion(v []int, s string) int {
	w, _ := parseRackVersion(s)
	return slices.Compare(v, w)
}

func parseRackVersion(v string) ([]int, bool) {
	v, _, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil, false
	}
	parts[2], _, _ = strings.Cut(parts[2], "rc")

	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		nums[i] = n
	}

	return nums, nums[0] == 3
}
