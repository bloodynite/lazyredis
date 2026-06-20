package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/bloodynite/lazyredis/internal/config"
)

const (
	formFieldName = iota
	formFieldAddr
	formFieldPassword
	formFieldDB
	formFieldMode
	formFieldMasterName
	formFieldSentinelPassword
	formFieldAddrs
	formFieldTLS
	formFieldProxy
	formFieldSSH
	formFieldSSHKey
	formFieldCount
)

var profileFormLabels = []string{
	"Name",
	"Addr",
	"Password",
	"DB",
	"Mode (standalone|cluster|sentinel)",
	"Master name (sentinel)",
	"Sentinel password",
	"Addrs (comma-sep, cluster/sentinel)",
	"TLS (off|on|skip)",
	"Proxy (http:// or socks5://)",
	"SSH (user@host:port)",
	"SSH private key",
}

func newProfileFormInputs() []textinput.Model {
	placeholders := []string{
		"name",
		"127.0.0.1:6379",
		"",
		"0",
		"standalone",
		"",
		"",
		"",
		"off",
		"",
		"",
		"~/.ssh/id_ed25519",
	}
	inputs := make([]textinput.Model, formFieldCount)
	for i := range inputs {
		inputs[i] = newFormInput(placeholders[i])
	}
	inputs[formFieldName].Focus()
	return inputs
}

func profileFromForm(values []textinput.Model) (config.Profile, error) {
	p := config.Profile{
		Name:     strings.TrimSpace(values[formFieldName].Value()),
		Addr:     strings.TrimSpace(values[formFieldAddr].Value()),
		Password: values[formFieldPassword].Value(),
		Mode:     config.Mode(strings.TrimSpace(values[formFieldMode].Value())),
		MasterName:       strings.TrimSpace(values[formFieldMasterName].Value()),
		SentinelPassword: values[formFieldSentinelPassword].Value(),
	}
	db, err := strconv.Atoi(strings.TrimSpace(values[formFieldDB].Value()))
	if err != nil || db < 0 {
		return p, fmt.Errorf("db must be a number >= 0")
	}
	p.DB = db
	if p.Mode == "" {
		p.Mode = config.ModeStandalone
	}

	addrs := config.ParseAddrs(values[formFieldAddrs].Value())
	if len(addrs) > 0 {
		p.Addrs = addrs
		if p.Addr == "" {
			p.Addr = addrs[0]
		}
	}

	tlsCfg, err := config.ParseTLSSpec(values[formFieldTLS].Value())
	if err != nil {
		return p, err
	}
	p.TLS = tlsCfg

	proxyCfg, err := config.ParseProxySpec(values[formFieldProxy].Value())
	if err != nil {
		return p, err
	}
	p.Proxy = proxyCfg

	sshUser, sshHost, err := config.ParseSSHSpec(values[formFieldSSH].Value())
	if err != nil {
		return p, err
	}
	if sshHost != "" {
		p.SSHTunnel = &config.SSHTunnel{
			Enabled:            true,
			User:               sshUser,
			Host:               sshHost,
			PrivateKey:         strings.TrimSpace(values[formFieldSSHKey].Value()),
			InsecureSkipVerify: true,
		}
	}

	return p, nil
}

func profileToFormValues(p config.Profile) []string {
	values := make([]string, formFieldCount)
	values[formFieldName] = p.Name
	values[formFieldAddr] = p.Addr
	values[formFieldPassword] = p.Password
	values[formFieldDB] = strconv.Itoa(p.DB)
	values[formFieldMode] = string(p.Mode)
	values[formFieldMasterName] = p.MasterName
	values[formFieldSentinelPassword] = p.SentinelPassword
	values[formFieldAddrs] = config.FormatAddrs(p.Addrs)
	values[formFieldTLS] = config.FormatTLSSpec(p.TLS)
	values[formFieldProxy] = config.FormatProxySpec(p.Proxy)
	values[formFieldSSH] = config.FormatSSHSpec(p.SSHTunnel)
	if p.SSHTunnel != nil {
		values[formFieldSSHKey] = p.SSHTunnel.PrivateKey
	}
	return values
}
