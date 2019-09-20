package common

import (
	"fmt"
	"time"
)

type Ticker func() error

func Tick(d time.Duration, fn Ticker) {
	if err := fn(); err != nil {
		fmt.Printf("ns=common at=tick error=%q\n", err)
	}

	for range time.Tick(d) {
		if err := fn(); err != nil {
			fmt.Printf("ns=common at=tick error=%q\n", err)
		}
	}
}
