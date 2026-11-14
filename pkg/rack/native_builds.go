package rack

import "strconv"

const nativeBuildParam = "karpenter_build_native_arch_enabled"

func (t Terraform) nativeBuildWarning() string {
	if t.provider != "aws" {
		return ""
	}

	vars, err := t.vars()
	if err != nil {
		return ""
	}

	return inertNativeBuildWarning(vars)
}

func inertNativeBuildWarning(vars map[string]string) string {
	on := func(key string) bool {
		b, _ := strconv.ParseBool(vars[key])
		return b
	}

	if !on(nativeBuildParam) || (vars["karpenter_enabled"] == "true" && on("build_node_enabled")) {
		return ""
	}

	return "WARNING: " + nativeBuildParam + " is set but has no effect. It requires karpenter_enabled=true and build_node_enabled=true.\n"
}
