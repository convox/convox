package common

import "fmt"

func Percent(cur, total float64) string {
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("%0.2f%%", (cur/total)*100)
}
