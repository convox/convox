package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/validate/content"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	sandboxVolumeName     = "sandbox-workspace"
	sandboxSeedVolumePath = "/convox-seed"
	sandboxFSGroup        = int64(1000)
)

var (
	// mountPathPattern keeps mount paths to characters that need no quoting anywhere they are used.
	mountPathPattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	sandboxVolumeMaxSize = resource.MustParse("100Gi")
)

type SandboxVolumeClaim struct {
	Name          string `json:"name"`
	Size          string `json:"size"`
	StorageClass  string `json:"storageClass,omitempty"`
	MountPath     string `json:"mountPath"`
	SeedFromImage bool   `json:"seedFromImage,omitempty"`

	quantity resource.Quantity
}

func parseSandboxVolumeClaims(raw string) ([]SandboxVolumeClaim, error) {
	var claims []SandboxVolumeClaim
	if err := json.Unmarshal([]byte(raw), &claims); err != nil {
		return nil, structs.ErrBadRequest("invalid volume claims: %s", err)
	}
	if len(claims) == 0 {
		return nil, nil
	}
	if len(claims) > 1 {
		return nil, structs.ErrBadRequest("at most one volume claim is supported")
	}
	for i := range claims {
		c := &claims[i]
		if msgs := content.IsDNS1123Label(c.Name); len(msgs) > 0 {
			return nil, structs.ErrBadRequest("invalid volume claim name %q: %s", c.Name, strings.Join(msgs, "; "))
		}
		q, err := resource.ParseQuantity(c.Size)
		if err != nil || q.Sign() <= 0 {
			return nil, structs.ErrBadRequest("invalid volume claim size %q", c.Size)
		}
		if q.Cmp(sandboxVolumeMaxSize) > 0 {
			return nil, structs.ErrBadRequest("volume claim size %q is over the %s limit", c.Size, sandboxVolumeMaxSize.String())
		}
		c.quantity = q
		if c.StorageClass != "" {
			if msgs := content.IsDNS1123Subdomain(c.StorageClass); len(msgs) > 0 {
				return nil, structs.ErrBadRequest("invalid storage class %q: %s", c.StorageClass, strings.Join(msgs, "; "))
			}
		}
		if !path.IsAbs(c.MountPath) || path.Clean(c.MountPath) != c.MountPath || c.MountPath == "/" || !mountPathPattern.MatchString(c.MountPath) {
			return nil, structs.ErrBadRequest("invalid volume claim mount path %q", c.MountPath)
		}
	}
	return claims, nil
}

func (p *Provider) ensureSandboxVolumeClaim(ctx context.Context, namespace, sandboxID string, c *SandboxVolumeClaim) error {
	pvcs := p.Cluster.CoreV1().PersistentVolumeClaims(namespace)

	existing, err := pvcs.Get(ctx, c.Name, am.GetOptions{})
	if err == nil {
		if existing.Labels[SandboxLabel] != "true" {
			return structs.ErrBadRequest("volume claim %q exists and is not a sandbox volume", c.Name)
		}
		if existing.Labels[SandboxIDLabel] != sandboxID {
			return structs.ErrBadRequest("volume claim %q belongs to another sandbox", c.Name)
		}
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return errors.WithStack(err)
	}

	pvc := &ac.PersistentVolumeClaim{
		ObjectMeta: am.ObjectMeta{
			Name: c.Name,
			Labels: map[string]string{
				SandboxLabel:   "true",
				SandboxIDLabel: sandboxID,
				"system":       "convox",
			},
		},
		Spec: ac.PersistentVolumeClaimSpec{
			AccessModes: []ac.PersistentVolumeAccessMode{ac.ReadWriteOnce},
			Resources: ac.VolumeResourceRequirements{
				Requests: ac.ResourceList{ac.ResourceStorage: c.quantity},
			},
		},
	}
	sc := c.StorageClass
	if sc == "" {
		sc = p.SandboxStorageClass
	}
	if sc != "" {
		pvc.Spec.StorageClassName = &sc
	}

	if _, err := pvcs.Create(ctx, pvc, am.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.WithStack(err)
	}
	return nil
}

func applySandboxVolumeClaim(s *ac.PodSpec, c *SandboxVolumeClaim) error {
	if len(s.Containers) == 0 {
		return fmt.Errorf("pod spec has no containers")
	}

	s.Volumes = append(s.Volumes, ac.Volume{
		Name: sandboxVolumeName,
		VolumeSource: ac.VolumeSource{
			PersistentVolumeClaim: &ac.PersistentVolumeClaimVolumeSource{ClaimName: c.Name},
		},
	})
	s.Containers[0].VolumeMounts = append(s.Containers[0].VolumeMounts, ac.VolumeMount{
		Name:      sandboxVolumeName,
		MountPath: c.MountPath,
	})

	if s.SecurityContext == nil {
		s.SecurityContext = &ac.PodSecurityContext{}
	}
	fsGroup := sandboxFSGroup
	policy := ac.FSGroupChangeOnRootMismatch
	s.SecurityContext.FSGroup = &fsGroup
	s.SecurityContext.FSGroupChangePolicy = &policy

	if c.SeedFromImage {
		main := s.Containers[0]
		script := `[ -e "$2/.convox/seeded" ] || { if [ -d "$1" ]; then cp -a "$1/." "$2/"; fi; mkdir -p "$2/.convox" && touch "$2/.convox/seeded"; }`
		s.InitContainers = append(s.InitContainers, ac.Container{
			Name:            "sandbox-seed",
			Image:           main.Image,
			ImagePullPolicy: main.ImagePullPolicy,
			Command:         []string{"sh", "-c", script, "sandbox-seed", c.MountPath, sandboxSeedVolumePath},
			Resources:       main.Resources,
			SecurityContext: main.SecurityContext,
			VolumeMounts: []ac.VolumeMount{{
				Name:      sandboxVolumeName,
				MountPath: sandboxSeedVolumePath,
			}},
		})
	}
	return nil
}

func (p *Provider) ProcessVolumeClaimDelete(app, name string) error {
	ns := p.AppNamespace(app)
	pvcs := p.Cluster.CoreV1().PersistentVolumeClaims(ns)

	pvc, err := pvcs.Get(context.TODO(), name, am.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}
	if pvc.Labels[SandboxLabel] != "true" {
		return structs.ErrBadRequest("volume claim %q is not a sandbox volume", name)
	}

	if err := pvcs.Delete(context.TODO(), name, am.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return errors.WithStack(err)
	}
	return nil
}
