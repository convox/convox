package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("certs", "list certificates", watch(Certs), stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.CertificateListOptions{}), flagRack, flagWatchInterval),
		Validate: stdcli.Args(0),
	})

	register("certs delete", "delete a certificate", CertsDelete, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<cert>",
		Validate: stdcli.Args(1),
	})

	register("certs dns", "list the dns records for certificates", CertsDns, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.BoolFlag("zone-file", "", "print records as zone file lines"),
		},
		Validate: stdcli.Args(0),
	})

	register("certs generate", "generate a certificate", CertsGenerate, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.CertificateGenerateOptions{}), flagId, flagRack),
		Usage:    "<domain> [domain...]",
		Validate: stdcli.ArgsMin(1),
	})

	register("certs import", "import a certificate", CertsImport, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagId,
			flagRack,
			stdcli.StringFlag("chain", "", "intermediate certificate chain"),
		},
		Usage:    "<pub> <key>",
		Validate: stdcli.Args(2),
	})

	register("certs migrate", "switch certificates to dns validation", CertsMigrate, stdcli.CommandOptions{
		Flags: []stdcli.Flag{flagRack, flagWait},
		Usage: "[cert...]",
	})

	register("certs renew", "renew a certificate", CertsRenew, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns route53 add", "configure letsencrypt dns route53 solver", CertLetsEncryptDnsRoute53Add, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("id", "", "dns solver id"),
			stdcli.StringFlag("dns-zones", "", "comma sperated dns zones"),
			stdcli.StringFlag("hosted-zone-id", "", "host zone id"),
			stdcli.StringFlag("role", "", "aws role arn to assume to access dns zones"),
			stdcli.StringFlag("region", "", "aws region"),
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns route53 update", "update letsencrypt dns route53 solver", CertLetsEncryptDnsRoute53Update, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("id", "", "dns solver id"),
			stdcli.StringFlag("dns-zones", "", "comma sperated dns zones"),
			stdcli.StringFlag("hosted-zone-id", "", "host zone id"),
			stdcli.StringFlag("role", "", "aws role arn to assume to access dns zones"),
			stdcli.StringFlag("region", "", "aws region"),
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns route53 delete", "delete letsencrypt dns route53 solver", CertLetsEncryptDnsRoute53Delete, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("id", "", "dns solver id"),
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns route53 list", "list letsencrypt dns route53 solver", CertLetsEncryptDnsRoute53List, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns route53 role", "letsencrypt dns route53 role arn", CertLetsEncryptDnsRoute53Role, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns cloudflare add", "configure letsencrypt dns cloudflare solver", CertLetsEncryptDnsCloudflareAdd, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("id", "", "dns solver id"),
			stdcli.StringFlag("dns-zones", "", "comma separated dns zones"),
			stdcli.StringFlag("api-token", "", "cloudflare api token value"),
			stdcli.StringFlag("api-key", "", "cloudflare api key value"),
			stdcli.StringFlag("email", "", "cloudflare account email (required for api key auth)"),
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns cloudflare update", "update letsencrypt dns cloudflare solver", CertLetsEncryptDnsCloudflareUpdate, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("id", "", "dns solver id"),
			stdcli.StringFlag("dns-zones", "", "comma separated dns zones"),
			stdcli.StringFlag("api-token", "", "cloudflare api token value"),
			stdcli.StringFlag("api-key", "", "cloudflare api key value"),
			stdcli.StringFlag("email", "", "cloudflare account email (required for api key auth)"),
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns cloudflare delete", "delete letsencrypt dns cloudflare solver", CertLetsEncryptDnsCloudflareDelete, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("id", "", "dns solver id"),
		},
		Validate: stdcli.Args(0),
	})

	register("letsencrypt dns cloudflare list", "list letsencrypt dns cloudflare solver", CertLetsEncryptDnsCloudflareList, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
		},
		Validate: stdcli.Args(0),
	})
}

func optionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return options.String(v)
}

func cloudflareCredentialFromCLI(id int, c *stdcli.Context) (*structs.Cloudflare, error) {
	tokenValue := strings.TrimSpace(c.String("api-token"))
	keyValue := strings.TrimSpace(c.String("api-key"))
	emailValue := strings.TrimSpace(c.String("email"))

	if tokenValue != "" && keyValue != "" {
		return nil, fmt.Errorf("cloudflare api token and api key options are mutually exclusive")
	}

	if tokenValue == "" && keyValue == "" && emailValue == "" {
		return nil, nil
	}

	if tokenValue == "" && keyValue == "" {
		return nil, fmt.Errorf("either --api-token or --api-key must be provided")
	}

	secretName := fmt.Sprintf("cloudflare-dns-credential-%d", id)
	cf := &structs.Cloudflare{}

	if tokenValue != "" {
		cf.ApiTokenValue = options.String(tokenValue)
		cf.ApiTokenSecretRefName = options.String(secretName)
		cf.ApiTokenSecretRefKey = options.String("api-token")
		cf.Email = nil
	} else {
		if emailValue == "" {
			return nil, fmt.Errorf("email is required when using --api-key")
		}
		cf.ApiKeyValue = options.String(keyValue)
		cf.ApiKeySecretRefName = options.String(secretName)
		cf.ApiKeySecretRefKey = options.String("api-key")
		cf.Email = options.String(emailValue)
	}

	return cf, nil
}

func Certs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.CertificateListOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	cs, err := rack.CertificateList(opts)
	if err != nil {
		return err
	}

	t := c.Table("ID", "DOMAIN", "EXPIRES", "Status")

	for _, c := range cs {
		t.AddRow(c.Id, c.Domain, common.Ago(c.Expiration), common.CoalesceString(c.Status, "Ready"))
	}

	if err := t.Print(); err != nil {
		return err
	}

	if lines := certsEmailLines(cs); len(lines) > 0 {
		return certsWrite(c.Writer().Stdout, append([]string{""}, lines...)...)
	}

	return nil
}

func CertsDelete(rack sdk.Interface, c *stdcli.Context) error {
	cert := c.Arg(0)

	c.Startf("Deleting certificate <id>%s</id>", cert)

	if err := rack.CertificateDelete(cert); err != nil {
		return err
	}

	return c.OK()
}

func CertsDns(rack sdk.Interface, c *stdcli.Context) error {
	d, err := rack.CertificateDnsClassic()
	if err != nil {
		return certsRackError("dns", err)
	}

	if d == nil {
		d = &structs.CertificateDns{}
	}

	if c.Bool("zone-file") {
		return certsZoneFile(c, d)
	}

	cs, err := rack.CertificateList(structs.CertificateListOptions{})
	if err != nil {
		return err
	}

	lines := append([]string{certsDnsSummary(d.Records)}, certsDnsLines(d, cs)...)

	if len(d.Records) > 0 {
		t := c.Table("ZONE", "NAME", "TYPE", "VALUE", "CHECK", "CERTIFICATES")

		for _, r := range d.Records {
			t.AddRow(r.Zone, r.Name, r.Type, r.Value, r.Check, strings.Join(r.Certificates, ","))
		}

		if err := t.Print(); err != nil {
			return err
		}

		lines = append([]string{""}, lines...)
	}

	return certsWrite(c.Writer().Stdout, lines...)
}

func CertsGenerate(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	var opts structs.CertificateGenerateOptions
	if err := c.Options(&opts); err != nil {
		return err
	}

	opts.Validation = options.String("dns")

	c.Startf("Generating certificate")

	cr, err := rack.CertificateGenerate(c.Args, opts)
	if err != nil {
		return err
	}

	c.OK(cr.Id)

	if d, err := rack.CertificateDnsClassic(); err == nil && d != nil {
		if err := certsGenerateRecords(c, d, cr.Id); err != nil {
			return err
		}
	}

	if c.Bool("id") {
		fmt.Fprintf(stdout, "%s", cr.Id)
	}

	return nil
}

func CertsImport(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	pub, err := os.ReadFile(c.Arg(0))
	if err != nil {
		return err
	}

	key, err := os.ReadFile(c.Arg(1))
	if err != nil {
		return err
	}

	var opts structs.CertificateCreateOptions

	if cf := c.String("chain"); cf != "" {
		chain, err := os.ReadFile(cf)
		if err != nil {
			return err
		}

		opts.Chain = options.String(string(chain))
	}

	c.Startf("Importing certificate")

	var cr *structs.Certificate

	if s.Version <= "20180708231844" {
		cr, err = rack.CertificateCreateClassic(string(pub), string(key), opts)
		if err != nil {
			return err
		}
	} else {
		cr, err = rack.CertificateCreate(string(pub), string(key), opts)
		if err != nil {
			return err
		}
	}

	c.OK(cr.Id)

	if c.Bool("id") {
		fmt.Fprintf(stdout, "%s", cr.Id)
	}

	return nil
}

func CertsMigrate(rack sdk.Interface, c *stdcli.Context) error {
	var ids []string

	if len(c.Args) > 0 {
		ids = c.Args
	}

	ms, err := rack.CertificateMigrateClassic(ids)
	if err != nil {
		return certsRackError("migrate", err)
	}

	if err := certsMigrationTable(c, ms); err != nil {
		return err
	}

	if !c.Bool("wait") {
		return nil
	}

	return certsMigrateWait(rack, c, ms)
}

func CertsRenew(rack sdk.Interface, c *stdcli.Context) error {
	app := app(c)
	c.Startf("Renewing certificate <app>%s</app>", app)

	if err := rack.CertificateRenew(app); err != nil {
		return err
	}

	return c.OK()
}

func CertLetsEncryptDnsRoute53Add(rack sdk.Interface, c *stdcli.Context) error {
	solver := &structs.Dns01Solver{}
	solver.Id = c.Int("id")
	if solver.Id <= 0 {
		return fmt.Errorf("invalid id or id is not provided")
	}
	for _, d := range strings.Split(c.String("dns-zones"), ",") {
		dd := strings.TrimSpace(d)
		if len(dd) > 0 {
			solver.DnsZones = append(solver.DnsZones, dd)
		}
	}
	solver.Route53 = &structs.Route53{}
	solver.Route53.Role = options.String(c.String("role"))
	solver.Route53.Region = options.String(c.String("region"))
	solver.Route53.HostedZoneID = options.String(c.String("hosted-zone-id"))

	if err := solver.Validate(); err != nil {
		return err
	}

	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	exists := map[int]bool{}
	for i := range config.Solvers {
		exists[config.Solvers[i].Id] = true
	}

	if exists[solver.Id] {
		return fmt.Errorf("provided id is already in use")
	}

	config.Solvers = append(config.Solvers, solver)

	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
}

func CertLetsEncryptDnsCloudflareAdd(rack sdk.Interface, c *stdcli.Context) error {
	solver := &structs.Dns01Solver{}
	solver.Id = c.Int("id")
	if solver.Id <= 0 {
		return fmt.Errorf("invalid id or id is not provided")
	}

	zonesInput := strings.TrimSpace(c.String("dns-zones"))
	if zonesInput == "" {
		return fmt.Errorf("dns zones are required")
	}

	for _, d := range strings.Split(zonesInput, ",") {
		dd := strings.TrimSpace(d)
		if dd != "" {
			solver.DnsZones = append(solver.DnsZones, dd)
		}
	}

	if len(solver.DnsZones) == 0 {
		return fmt.Errorf("dns zones are required")
	}

	cf, err := cloudflareCredentialFromCLI(solver.Id, c)
	if err != nil {
		return err
	}
	if cf == nil {
		return fmt.Errorf("either --api-token or --api-key must be provided")
	}
	solver.Cloudflare = cf

	if err := solver.Validate(); err != nil {
		return err
	}

	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	exists := map[int]bool{}
	for i := range config.Solvers {
		exists[config.Solvers[i].Id] = true
	}

	if exists[solver.Id] {
		return fmt.Errorf("provided id is already in use")
	}

	config.Solvers = append(config.Solvers, solver)

	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
}

func CertLetsEncryptDnsCloudflareUpdate(rack sdk.Interface, c *stdcli.Context) error {
	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	id := c.Int("id")
	if id <= 0 {
		return fmt.Errorf("invalid id or id is not provided")
	}

	var solver *structs.Dns01Solver
	for _, s := range config.Solvers {
		if s.Id == id {
			solver = s
			break
		}
	}

	if solver == nil {
		return fmt.Errorf("solver with id %d not found", id)
	}

	if dz := strings.TrimSpace(c.String("dns-zones")); dz != "" {
		zones := []string{}
		for _, d := range strings.Split(dz, ",") {
			dd := strings.TrimSpace(d)
			if dd != "" {
				zones = append(zones, dd)
			}
		}
		if len(zones) == 0 {
			return fmt.Errorf("dns zones are required")
		}
		solver.DnsZones = zones
	}

	cf, err := cloudflareCredentialFromCLI(solver.Id, c)
	if err != nil {
		return err
	}
	if cf != nil {
		solver.Cloudflare = cf
	}

	if err := solver.Validate(); err != nil {
		return err
	}

	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
}

func CertLetsEncryptDnsCloudflareDelete(rack sdk.Interface, c *stdcli.Context) error {
	id := c.Int("id")
	if id <= 0 {
		return fmt.Errorf("invalid id or id is not provided")
	}

	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	found := false
	solvers := []*structs.Dns01Solver{}
	for i := range config.Solvers {
		if config.Solvers[i].Id == id {
			found = true
		} else {
			solvers = append(solvers, config.Solvers[i])
		}
	}

	if !found {
		return fmt.Errorf("solver with id %d not found", id)
	}

	config.Solvers = solvers
	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
}

func CertLetsEncryptDnsCloudflareList(rack sdk.Interface, c *stdcli.Context) error {
	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	t := c.Table("ID", "DNS-ZONES", "TYPE", "SECRET", "FIELDS", "EMAIL")

	for _, solver := range config.Solvers {
		if solver.Cloudflare != nil {
			cl := solver.Cloudflare

			authType := ""
			secretName := ""
			fields := []string{}

			if cl.ApiTokenSecretRefName != nil && *cl.ApiTokenSecretRefName != "" {
				authType = "API Token"
				secretName = *cl.ApiTokenSecretRefName
				if cl.ApiTokenSecretRefKey != nil && *cl.ApiTokenSecretRefKey != "" {
					fields = append(fields, *cl.ApiTokenSecretRefKey)
				}
			} else if cl.ApiKeySecretRefName != nil && *cl.ApiKeySecretRefName != "" {
				authType = "API Key"
				secretName = *cl.ApiKeySecretRefName
				if cl.ApiKeySecretRefKey != nil && *cl.ApiKeySecretRefKey != "" {
					fields = append(fields, *cl.ApiKeySecretRefKey)
				}
				fields = append(fields, "email")
			}

			t.AddRow(
				strconv.Itoa(solver.Id),
				strings.Join(solver.DnsZones, ","),
				authType,
				secretName,
				strings.Join(fields, ", "),
				options.StringValueSafe(cl.Email),
			)
		}
	}

	return t.Print()
}

func CertLetsEncryptDnsRoute53Update(rack sdk.Interface, c *stdcli.Context) error {
	solver := &structs.Dns01Solver{}
	solver.Id = c.Int("id")
	for _, d := range strings.Split(c.String("dns-zones"), ",") {
		dd := strings.TrimSpace(d)
		if len(dd) > 0 {
			solver.DnsZones = append(solver.DnsZones, dd)
		}
	}
	solver.Route53 = &structs.Route53{}
	solver.Route53.Role = options.String(c.String("role"))
	solver.Route53.Region = options.String(c.String("region"))
	solver.Route53.HostedZoneID = options.String(c.String("hosted-zone-id"))

	if err := solver.Validate(); err != nil {
		return err
	}

	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	found := false
	for i := range config.Solvers {
		if config.Solvers[i].Id == solver.Id {
			config.Solvers[i] = solver
			found = true
		}
	}

	if !found {
		return fmt.Errorf("not found or invalid id")
	}

	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
}

func CertLetsEncryptDnsRoute53Delete(rack sdk.Interface, c *stdcli.Context) error {
	id := c.Int("id")
	if id <= 0 {
		return fmt.Errorf("invalid id or id is not provided")
	}

	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	found := false
	solvers := []*structs.Dns01Solver{}
	for i := range config.Solvers {
		if config.Solvers[i].Id == id {
			found = true
		} else {
			solvers = append(solvers, config.Solvers[i])
		}
	}

	if !found {
		return fmt.Errorf("not found or invalid id")
	}

	config.Solvers = solvers
	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
}

func CertLetsEncryptDnsRoute53List(rack sdk.Interface, c *stdcli.Context) error {
	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	t := c.Table("ID", "DNS-ZONES", "HOSTED-ZONE-ID", "REGION", "ROLE")

	for _, c := range config.Solvers {
		if c.Route53 != nil {
			t.AddRow(strconv.Itoa(c.Id), strings.Join(c.DnsZones, ","),
				options.StringValueSafe(c.Route53.HostedZoneID),
				options.StringValueSafe(c.Route53.Region),
				options.StringValueSafe(c.Route53.Role),
			)
		}
	}

	return t.Print()
}

func CertLetsEncryptDnsRoute53Role(rack sdk.Interface, c *stdcli.Context) error {
	config, err := rack.LetsEncryptConfigGet()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}

	return c.Writef("%s\n", config.Role)
}

func certsEmailLines(cs structs.Certificates) []string {
	custom, site := 0, 0

	for _, cert := range cs {
		if !cert.Owned || cert.Status != "email" {
			continue
		}

		if certsConvoxSite(cert.Domain) {
			site++
		} else {
			custom++
		}
	}

	lines := []string{}

	switch {
	case custom == 1:
		lines = append(lines, "1 certificate uses email validation, which stops renewing on 2027-09-30: run convox certs dns")
	case custom > 1:
		lines = append(lines, fmt.Sprintf("%d certificates use email validation, which stops renewing on 2027-09-30: run convox certs dns", custom))
	}

	switch {
	case site == 1:
		lines = append(lines, "1 convox.site certificate is still on email validation: run convox certs migrate to see why")
	case site > 1:
		lines = append(lines, fmt.Sprintf("%d convox.site certificates are still on email validation: run convox certs migrate to see why", site))
	}

	return lines
}

func certsConvoxSite(domain string) bool {
	return strings.HasSuffix(strings.TrimPrefix(strings.ToLower(domain), "*."), ".convox.site")
}

func certsDnsSummary(rs []structs.CertificateDnsRecord) string {
	summary := fmt.Sprintf("%d records", len(rs))

	if len(rs) == 1 {
		summary = "1 record"
	}

	counts := []string{}

	for _, check := range []string{"ok", "missing", "wrong", "unknown"} {
		n := 0

		for _, r := range rs {
			if r.Check == check {
				n++
			}
		}

		if n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", n, check))
		}
	}

	if len(counts) == 0 {
		return summary
	}

	return fmt.Sprintf("%s: %s", summary, strings.Join(counts, ", "))
}

func certsDnsLines(d *structs.CertificateDns, cs structs.Certificates) []string {
	lines := []string{}
	zones := map[string]bool{}

	for _, r := range d.Records {
		if r.Route53 && !zones[r.Zone] {
			zones[r.Zone] = true
			lines = append(lines, fmt.Sprintf("%s is a Route 53 zone in this AWS account. To have the rack add its records: convox rack params set Route53CertificateValidation=Yes", r.Zone))
		}
	}

	lines = append(lines, certsDnsNotes(d)...)

	unused := []string{}

	for _, cert := range cs {
		if !cert.Owned && cert.Status == "email" && !certsConvoxSite(cert.Domain) {
			unused = append(unused, cert.Id)
		}
	}

	switch {
	case len(unused) == 1:
		lines = append(lines, fmt.Sprintf("%s uses email validation but is not used by this rack, so it leaves it alone", unused[0]))
	case len(unused) > 1:
		lines = append(lines, fmt.Sprintf("%s use email validation but are not used by this rack, so it leaves them alone", strings.Join(unused, ", ")))
	}

	for _, r := range d.Records {
		if r.Check != "ok" {
			return append(lines, "When every record shows ok: convox certs migrate --wait")
		}
	}

	return lines
}

func certsDnsNotes(d *structs.CertificateDns) []string {
	lines := []string{}

	switch n := len(d.Preparing); {
	case n == 1:
		lines = append(lines, "1 certificate is still being prepared, run again in a few minutes")
	case n > 1:
		lines = append(lines, fmt.Sprintf("%d certificates are still being prepared, run again in a few minutes", n))
	}

	for _, e := range d.Errors {
		lines = append(lines, fmt.Sprintf("%s: %s", e.Id, e.Error))
	}

	return lines
}

func certsZoneFile(c *stdcli.Context, d *structs.CertificateDns) error {
	lines := []string{}
	zone := ""

	for i, r := range d.Records {
		if i == 0 || r.Zone != zone {
			zone = r.Zone
			lines = append(lines, fmt.Sprintf("; zone %s", zone))
		}

		lines = append(lines, fmt.Sprintf("%s 300 IN CNAME %s", r.Name, r.Value))
	}

	if err := certsWrite(c.Writer().Stdout, lines...); err != nil {
		return err
	}

	return certsWrite(c.Writer().Stderr, certsDnsNotes(d)...)
}

func certsGenerateRecords(c *stdcli.Context, d *structs.CertificateDns, id string) error {
	t := c.Table("ZONE", "NAME", "TYPE", "VALUE", "CHECK")

	for _, r := range d.Records {
		if slices.Contains(r.Certificates, id) {
			t.AddRow(r.Zone, r.Name, r.Type, r.Value, r.Check)
		}
	}

	if len(t.Rows) == 0 {
		return certsWrite(c.Writer().Stdout, "Run convox certs dns in a minute to see the records to add")
	}

	if err := t.Print(); err != nil {
		return err
	}

	return certsWrite(c.Writer().Stdout, fmt.Sprintf("Deploy the domain once convox certs shows %s as dns", id))
}

func certsMigrationTable(c *stdcli.Context, ms structs.CertificateMigrations) error {
	t := c.Table("CERTIFICATE", "DOMAIN", "RESULT")

	for _, m := range ms {
		t.AddRow(m.Id, m.Domain, certsMigrationResult(m))
	}

	return t.Print()
}

func certsMigrationResult(m structs.CertificateMigration) string {
	switch m.Result {
	case "waiting":
		return "waiting for its DNS record (run convox certs dns)"
	case "error":
		return fmt.Sprintf("error: %s", m.Detail)
	}

	return m.Result
}

func certsMigrateWait(rack sdk.Interface, c *stdcli.Context, ms structs.CertificateMigrations) error {
	tracked := structs.CertificateMigrations{}

	for _, m := range ms {
		switch m.Result {
		case "preparing", "publishing", "switching":
			tracked = append(tracked, m)
		}
	}

	if len(tracked) == 0 {
		return nil
	}

	c.Startf("Waiting for switches to finish")

	tracked, err := certsMigrationPoll(rack, tracked)
	if err != nil {
		return err
	}

	unfinished := []string{}

	for _, m := range tracked {
		if m.Result != "dns" {
			unfinished = append(unfinished, m.Id)
		}
	}

	if len(unfinished) > 0 {
		if err := certsWrite(c.Writer().Stdout, ""); err != nil {
			return err
		}

		if err := certsMigrationTable(c, tracked); err != nil {
			return err
		}

		return fmt.Errorf("certificates did not finish switching: %s", strings.Join(unfinished, ", "))
	}

	if err := c.OK(); err != nil {
		return err
	}

	return certsMigrationTable(c, tracked)
}

func certsMigrationPoll(rack sdk.Interface, tracked structs.CertificateMigrations) (structs.CertificateMigrations, error) {
	deadline := time.Now().Add(CertsWaitTimeout)

	for time.Now().Before(deadline) {
		ids := []string{}

		for _, m := range tracked {
			if !certsMigrationDone(m.Result) {
				ids = append(ids, m.Id)
			}
		}

		if len(ids) == 0 {
			break
		}

		time.Sleep(CertsWaitInterval)

		ms, err := rack.CertificateMigrationListClassic(ids)
		if certsRouteMissing(err) {
			return nil, certsRackError("migrate", err)
		}
		if err != nil {
			continue
		}

		for _, m := range ms {
			for i := range tracked {
				if tracked[i].Id == m.Id {
					tracked[i] = m
				}
			}
		}
	}

	return tracked, nil
}

func certsMigrationDone(result string) bool {
	switch result {
	case "dns", "error", "failed", "waiting":
		return true
	}

	return false
}

func certsRackError(command string, err error) error {
	if certsRouteMissing(err) {
		return fmt.Errorf("convox certs %s requires a rack on the latest release", command)
	}

	return err
}

func certsRouteMissing(err error) bool {
	if err == nil {
		return false
	}

	for _, s := range []string{"response status 404", "response status 405", "404 page not found"} {
		if strings.Contains(err.Error(), s) {
			return true
		}
	}

	return false
}

func certsWrite(w io.Writer, lines ...string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	return nil
}
