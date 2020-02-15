package common

import (
	"flag"

	"k8s.io/klog"
)

func InitializeKlog() {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(fs)
	_ = fs.Parse([]string{"-skip_headers"})
}
