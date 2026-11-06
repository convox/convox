package common

import "strings"

func UpperName(name string) string {
	if name == "" {
		return ""
	}

	// replace underscores with dashes
	name = strings.Replace(name, "_", "-", -1)

	// myapp -> Myapp; my-app -> MyApp
	us := strings.ToUpper(name[0:1]) + name[1:]

	for {
		i := strings.Index(us, "-")

		if i == -1 {
			break
		}

		s := us[0:i]

		if len(us) > i+1 {
			s += strings.ToUpper(us[i+1 : i+2])
		}

		if len(us) > i+2 {
			s += us[i+2:]
		}

		us = s
	}

	return us
}

func ContainsInStringSlice(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
