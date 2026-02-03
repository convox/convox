package manifest

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
	kyaml "sigs.k8s.io/yaml"
)

type DefaultsSetter interface {
	SetDefaults() error
}

type NameGetter interface {
	GetName() string
}

type NameSetter interface {
	SetName(name string) error
}

func (v Balancers) MarshalYAML() (interface{}, error) {
	return marshalMapSlice(v)
}

func (v *Balancers) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalMapSlice(unmarshal, v)
}

func (v *Balancer) SetName(name string) error {
	v.Name = name
	return nil
}

func (v BalancerPorts) MarshalYAML() (interface{}, error) {
	return marshalMapSlice(v)
}

func (v *BalancerPorts) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalMapSlice(unmarshal, v)
}

func (v *BalancerPort) SetName(name string) error {
	i, err := strconv.Atoi(name)
	if err != nil {
		return err
	}

	v.Source = i

	return nil
}

func (v *BalancerPort) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case map[interface{}]interface{}:
		type balancerPort BalancerPort
		var bp balancerPort
		if err := remarshal(w, &bp); err != nil {
			return err
		}
		v.Protocol = bp.Protocol
		v.Target = bp.Target
	case int:
		v.Protocol = "TCP"
		v.Target = t
	}

	return nil
}

func (v *BalancerWhitelist) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case []interface{}:
		*v = []string{}
		for _, s := range t {
			switch st := s.(type) {
			case string:
				*v = append(*v, st)
			default:
				return fmt.Errorf("unknown type for balancer whitelist: %T", t)
			}
		}
	case string:
		*v = []string{t}
	default:
		return fmt.Errorf("unknown type for balancer whitelist: %T", t)
	}

	return nil
}

func (v *Environment) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case []interface{}:
		for _, s := range t {
			switch st := s.(type) {
			case []interface{}:
				for _, stv := range st {
					if sv, ok := stv.(string); ok {
						*v = append(*v, sv)
					}
				}
			case string:
				*v = append(*v, st)
			}
		}
	default:
		return fmt.Errorf("unknown type for service environment: %T", t)
	}

	return nil
}

func (v Resources) MarshalYAML() (interface{}, error) {
	return marshalMapSlice(v)
}

func (v *Resources) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalMapSlice(unmarshal, v)
}

func (v *Resource) SetName(name string) error {
	v.Name = name
	return nil
}

func (v Services) MarshalYAML() (interface{}, error) {
	return marshalMapSlice(v)
}

func (v *Services) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalMapSlice(unmarshal, v)
}

func (v *Service) SetName(name string) error {
	v.Name = name
	return nil
}

func (a *Annotations) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawAnnotations []interface{}
	if err := unmarshal(&rawAnnotations); err != nil {
		return fmt.Errorf("failed to unmarshal annotations: %w", err)
	}

	var annotations []Annotation

	for _, raw := range rawAnnotations {
		switch item := raw.(type) {
		case string:
			parts := strings.SplitN(item, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid annotation format: %s", item)
			}
			annotations = append(annotations, Annotation{Key: parts[0], Value: parts[1]})

		case map[interface{}]interface{}:
			for k, v := range item {
				keyStr, ok := k.(string)
				if !ok {
					return fmt.Errorf("annotation key is not a string: %v", k)
				}
				vStr, ok := v.(string)
				if !ok {
					return fmt.Errorf("annotation value is not a string: %v", k)
				}
				annotations = append(annotations, Annotation{Key: keyStr, Value: vStr})
			}

		case map[string]interface{}:
			for k, v := range item {
				vStr, ok := v.(string)
				if !ok {
					return fmt.Errorf("annotation value is not a string: %v", k)
				}
				annotations = append(annotations, Annotation{Key: k, Value: vStr})
			}

		default:
			return fmt.Errorf("unsupported annotation type: %T", item)
		}
	}

	*a = annotations
	return nil
}

func (v *ServiceAgent) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case bool:
		v.Enabled = t
	case map[interface{}]interface{}:
		return fmt.Errorf("agent ports are now specified at the service level")
	default:
		return fmt.Errorf("could not parse agent: %+v", w)
	}

	return nil
}

func (v *ServiceBuild) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case map[interface{}]interface{}:
		type serviceBuild ServiceBuild
		var r serviceBuild
		if err := remarshal(w, &r); err != nil {
			return err
		}
		v.Args = r.Args
		v.Manifest = r.Manifest
		v.Path = r.Path
	case string:
		v.Path = t
	default:
		return fmt.Errorf("unknown type for service build: %T", t)
	}

	return nil
}

func (v ServiceBuild) MarshalYAML() (interface{}, error) {
	if len(v.Args) == 0 {
		return v.Path, nil
	}

	return v, nil
}

func (v *ServiceDomains) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case []interface{}:
		for _, s := range t {
			switch st := s.(type) {
			case string:
				if tst := strings.TrimSpace(st); tst != "" {
					*v = append(*v, tst)
				}
			default:
				return fmt.Errorf("unknown type for service domain: %T", s)
			}
		}
	case string:
		for _, d := range strings.Split(t, ",") {
			if td := strings.TrimSpace(d); td != "" {
				*v = append(*v, td)
			}
		}
	default:
		return fmt.Errorf("unknown type for service domain: %T", t)
	}

	return nil
}

func (v *ServiceHealth) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case map[interface{}]interface{}:
		if w, ok := t["grace"].(int); ok {
			v.Grace = w
		}
		if w, ok := t["path"].(string); ok {
			v.Path = w
		}
		if w, ok := t["interval"].(int); ok {
			v.Interval = w
		}
		if w, ok := t["timeout"].(int); ok {
			v.Timeout = w
		}
		if w, ok := t["disable"].(bool); ok {
			v.Disable = w
		}
	case string:
		v.Path = t
	default:
		return fmt.Errorf("unknown type for service health: %T", t)
	}

	return nil
}

func (v *ServicePortProtocol) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case string:
		ps := strings.Split(t, "/")
		pi, err := strconv.Atoi(ps[0])
		if err != nil {
			return err
		}
		v.Port = pi
		if len(ps) > 1 {
			v.Protocol = ps[1]
		}
	case int:
		v.Port = t
		v.Protocol = "tcp"
	default:
		return fmt.Errorf("invalid port: %s", t)
	}

	return nil
}

func (v *ServicePortScheme) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case map[interface{}]interface{}:
		switch u := t["port"].(type) {
		case int:
			v.Port = u
		case string:
			ps := strings.Split(u, ":")
			pp := ps[0]
			if len(ps) > 1 {
				v.Scheme = ps[0]
				pp = ps[1]
			}
			pi, err := strconv.Atoi(pp)
			if err != nil {
				return err
			}
			v.Port = pi
		case nil:
		default:
			return fmt.Errorf("could not parse port: %s", t)
		}

		if scheme := t["scheme"]; scheme != nil {
			v.Scheme = scheme.(string)
		}

		if v.Port == 0 {
			return fmt.Errorf("could not parse port: %+v", t)
		}
	case string:
		ps := strings.Split(t, ":")
		pp := ps[0]
		if len(ps) > 1 {
			v.Scheme = ps[0]
			pp = ps[1]
		}
		pi, err := strconv.Atoi(pp)
		if err != nil {
			return err
		}
		v.Port = pi
	case int:
		v.Port = t
		// v.Scheme = "http"
	default:
		return fmt.Errorf("invalid port: %s", t)
	}

	if v.Scheme == "grpc" {
		v.Scheme = "GRPC"
	}

	return nil
}

func (v ServicePortScheme) MarshalYAML() (interface{}, error) {
	if v.Port == 0 {
		return nil, nil
	}

	return v, nil
}

func (v *ServiceScale) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case int:
		v.Count = ServiceScaleCount{Min: t, Max: t}
	case string:
		var c ServiceScaleCount
		if err := remarshal(w, &c); err != nil {
			return err
		}
		v.Count = c
	case map[interface{}]interface{}:
		if w, ok := t["count"].(interface{}); ok {
			var c ServiceScaleCount
			if err := remarshal(w, &c); err != nil {
				return err
			}
			v.Count = c
		}
		if w, ok := t["cpu"].(int); ok {
			v.Cpu = w
		}
		if w, ok := t["gpu"].(interface{}); ok {
			var g ServiceScaleGpu
			if err := remarshal(w, &g); err != nil {
				return err
			}
			v.Gpu = g
		}
		if w, ok := t["memory"].(int); ok {
			v.Memory = w
		}
		if w, ok := t["targets"].(interface{}); ok {
			var t ServiceScaleTargets
			if err := remarshal(w, &t); err != nil {
				return err
			}
			v.Targets = t
		}
		if w, ok := t["keda"].(interface{}); ok {
			var k ServiceScaleKeda
			if err := remarshalUsingKyaml(w, &k); err != nil {
				return err
			}
			v.Keda = &k
		}
		if w, ok := t["vpa"].(interface{}); ok {
			var k VPA
			if err := remarshal(w, &k); err != nil {
				return err
			}
			v.VPA = &k
		}
		if w, ok := t["limit"].(interface{}); ok {
			var lmt ServiceResourceLimit
			if err := remarshal(w, &lmt); err != nil {
				return err
			}
			v.Limit = lmt
		}
	default:
		return fmt.Errorf("unknown type for service scale: %T", t)
	}

	return nil
}

func (v *ServiceScaleGpu) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case map[interface{}]interface{}:
		if w, ok := t["count"].(int); ok {
			v.Count = w
		}
		if w, ok := t["vendor"].(string); ok {
			v.Vendor = w
		}
	case int:
		v.Count = t
	default:
		return fmt.Errorf("unknown type for service scale gpu: %T", t)
	}

	return nil
}

func (v *ServiceScaleMetrics) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w map[string]ServiceScaleMetric

	if err := unmarshal(&w); err != nil {
		return err
	}

	*v = ServiceScaleMetrics{}

	for wk, wv := range w {
		parts := strings.Split(wk, "/")
		wv.Namespace = strings.Join(parts[0:len(parts)-1], "/")
		wv.Name = parts[len(parts)-1]
		*v = append(*v, wv)
	}

	return nil
}

func (v *ServiceScaleCount) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var w interface{}

	if err := unmarshal(&w); err != nil {
		return err
	}

	switch t := w.(type) {
	case int:
		v.Min = t
		v.Max = t
	case string:
		parts := strings.Split(t, "-")

		switch len(parts) {
		case 1:
			i, err := strconv.Atoi(parts[0])
			if err != nil {
				return err
			}

			v.Min = i

			if !strings.HasSuffix(parts[0], "+") {
				v.Max = i
			}
		case 2:
			i, err := strconv.Atoi(parts[0])
			if err != nil {
				return err
			}

			j, err := strconv.Atoi(parts[1])
			if err != nil {
				return err
			}

			v.Min = i
			v.Max = j
		default:
			return fmt.Errorf("invalid scale: %v", w)
		}
	case map[interface{}]interface{}:
		if min := t["min"]; min != nil {
			switch u := min.(type) {
			case int:
				v.Min = u
			case string:
				mins, err := strconv.Atoi(u)
				if err != nil {
					return err
				}
				v.Min = mins
			default:
				return fmt.Errorf("invalid scale: %v", w)
			}
		}
		if max := t["max"]; max != nil {
			switch u := max.(type) {
			case int:
				v.Max = u
			case string:
				maxs, err := strconv.Atoi(u)
				if err != nil {
					return err
				}
				v.Max = maxs
			default:
				return fmt.Errorf("invalid scale: %v", w)
			}
		}
	default:
		return fmt.Errorf("invalid scale: %v", w)
	}

	return nil
}

func (v ServiceScaleCount) MarshalYAML() (interface{}, error) {
	if v.Min == v.Max {
		return v.Min, nil
	}

	return v, nil
}

func (v Timers) MarshalYAML() (interface{}, error) {
	return marshalMapSlice(v)
}

func (v *Timers) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalMapSlice(unmarshal, v)
}

func remarshal(in, out interface{}) error {
	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, out)
}

func remarshalUsingKyaml(in, out interface{}) error {
	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}

	return kyaml.Unmarshal(data, out)
}

func marshalMapSlice(in interface{}) (interface{}, error) {
	ms := yaml.MapSlice{}

	iv := reflect.ValueOf(in)

	if iv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("not a slice")
	}

	for i := 0; i < iv.Len(); i++ {
		ii := iv.Index(i).Interface()

		if iing, ok := ii.(NameGetter); ok {
			ms = append(ms, yaml.MapItem{
				Key:   iing.GetName(),
				Value: ii,
			})
		}
	}

	return ms, nil
}

func unmarshalMapSlice(unmarshal func(interface{}) error, v interface{}) error {
	rv := reflect.ValueOf(v).Elem()
	vit := rv.Type().Elem()

	var ms yaml.MapSlice

	if err := unmarshal(&ms); err != nil {
		return err
	}

	for _, msi := range ms {
		item := reflect.New(vit).Interface()

		if err := remarshal(msi.Value, item); err != nil {
			return err
		}

		if ds, ok := item.(DefaultsSetter); ok {
			if err := ds.SetDefaults(); err != nil {
				return err
			}
		}

		if ns, ok := item.(NameSetter); ok {
			switch t := msi.Key.(type) {
			case int:
				if err := ns.SetName(fmt.Sprintf("%d", t)); err != nil {
					return err
				}
			case string:
				if err := ns.SetName(t); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown key type: %T", t)
			}
		}

		rv.Set(reflect.Append(rv, reflect.ValueOf(item).Elem()))
	}

	return nil
}

func yamlAttributes(data []byte) (map[string]bool, error) {
	attrs := map[string]bool{}

	var v interface{}

	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	m, ok := v.(map[interface{}]interface{})
	if !ok {
		return attrs, nil
	}

	for ki, v := range m {
		k := ""

		switch t := ki.(type) {
		case string:
			k = t
		case int:
			k = strconv.Itoa(t)
		default:
			continue
		}

		attrs[k] = true

		vdata, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}

		vattrs, err := yamlAttributes(vdata)
		if err != nil {
			return nil, err
		}

		for vk := range vattrs {
			attrs[fmt.Sprintf("%s.%s", k, vk)] = true
		}
	}

	return attrs, nil
}
