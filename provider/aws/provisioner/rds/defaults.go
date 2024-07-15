package rds

import "fmt"

func (p *Provisioner) ApplyInstallDefaults(options map[string]string) error {
	var err error

	if _, has := options[ParamPort]; !has {
		options[ParamPort] = DefaultDbPort(options[ParamEngine])
	}

	if _, has := options[ParamMasterUserPassword]; !has {
		options[ParamMasterUserPassword], err = GenerateSecurePassword(36)
		if err != nil {
			return fmt.Errorf("failed to generate password: %s", err)
		}
	}
	return nil
}

func (p *Provisioner) ApplyRestoreFromSnapshotDefaults(options map[string]string) error {
	var err error

	if _, has := options[ParamPort]; !has {
		options[ParamPort] = DefaultDbPort(options[ParamEngine])
	}
	return err
}

func DefaultDbPort(engine string) string {
	switch engine {
	case "mysql", "mariadb":
		return "3306"
	case "postgres":
		return "5432"
	default:
		return "8080"
	}
}
