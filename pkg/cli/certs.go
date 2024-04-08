package cli

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("certs", "list certificates", watch(Certs), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagWatchInterval},
		Validate: stdcli.Args(0),
	})

	register("certs delete", "delete a certificate", CertsDelete, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<cert>",
		Validate: stdcli.Args(1),
	})

	register("certs generate", "generate a certificate", CertsGenerate, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagId, flagRack},
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
}

func Certs(rack sdk.Interface, c *stdcli.Context) error {
	cs, err := rack.CertificateList()
	if err != nil {
		return err
	}

	t := c.Table("ID", "DOMAIN", "EXPIRES")

	for _, c := range cs {
		t.AddRow(c.Id, c.Domain, common.Ago(c.Expiration))
	}

	return t.Print()
}

func CertsDelete(rack sdk.Interface, c *stdcli.Context) error {
	cert := c.Arg(0)

	c.Startf("Deleting certificate <id>%s</id>", cert)

	if err := rack.CertificateDelete(cert); err != nil {
		return err
	}

	return c.OK()
}

func CertsGenerate(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	c.Startf("Generating certificate")

	cr, err := rack.CertificateGenerate(c.Args)
	if err != nil {
		return err
	}

	c.OK(cr.Id)

	if c.Bool("id") {
		fmt.Fprintf(stdout, cr.Id)
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

	pub, err := ioutil.ReadFile(c.Arg(0))
	if err != nil {
		return err
	}

	key, err := ioutil.ReadFile(c.Arg(1))
	if err != nil {
		return err
	}

	var opts structs.CertificateCreateOptions

	if cf := c.String("chain"); cf != "" {
		chain, err := ioutil.ReadFile(cf)
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
		fmt.Fprintf(stdout, cr.Id)
	}

	return nil
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
		return fmt.Errorf("id required or provided id is already in use")
	}

	config.Solvers = append(config.Solvers, solver)

	if err := rack.LetsEncryptConfigApply(*config); err != nil {
		return err
	}
	return c.OK()
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
