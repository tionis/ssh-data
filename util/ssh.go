package util

import (
	"database/sql"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
	"time"
)

type AuthorizedKeyRaw struct {
	Key     ssh.PublicKey
	Options map[string]string
}

type AuthorizedKey struct {
	Key             ssh.PublicKey
	comment         string
	Principals      []string
	IsCA            bool
	command         sql.NullString
	environment     map[string]string
	expiryTime      sql.NullTime
	agentForwarding bool
	from            []*Pattern
	PortForwarding  bool
	pty             bool
	UserRC          bool
	X11Forwarding   bool
	permitListen    sql.NullString
	permitOpen      sql.NullString
	noTouchReq      bool
	verifyReq       bool
	tunnel          sql.NullString
}

func (k *AuthorizedKey) MatchesPrincipal(input string) bool {
	for _, p := range k.Principals {
		if p == input {
			return true
		}
	}
	return false
}

func ParseSSHTimespec(value string) (time.Time, error) {
	switch len(value) {
	case 8: // YYYYMMDD (using local timezone)
		return time.ParseInLocation("20060102", value, time.Local)
	case 9: // YYYYMMDDZ (using UTC)
		return time.Parse("20060102Z", value)
	case 12: // YYYYMMDDHHMM (using local timezone)
		return time.ParseInLocation("200601021504", value, time.Local)
	case 13: // YYYYMMDDHHMMZ (using UTC)
		return time.Parse("200601021504Z", value)
	case 15: // YYYYMMDDHHMMSS (using local timezone)
		return time.ParseInLocation("20060102150405", value, time.Local)
	case 16: // YYYYMMDDHHMMSSZ (using UTC)
		return time.Parse("20060102150405Z", value)
	default:
		return time.Time{}, fmt.Errorf("invalid timespec: %s", value)
	}
}

func NewAuthorizedKey(key ssh.PublicKey, comment string, options []string) (*AuthorizedKey, error) {
	ak := &AuthorizedKey{
		Key:        key,
		comment:    comment,
		Principals: []string{},
		IsCA:       false,
		command: sql.NullString{
			String: "",
			Valid:  false,
		},
		environment: map[string]string{},
		expiryTime: sql.NullTime{
			Time:  time.Time{},
			Valid: false,
		},
		agentForwarding: true,
		from:            []*Pattern{},
		PortForwarding:  true,
		pty:             true,
		UserRC:          true,
		X11Forwarding:   true,
		permitListen: sql.NullString{
			String: "",
			Valid:  false,
		},
		permitOpen: sql.NullString{
			String: "",
			Valid:  false,
		},
		noTouchReq: false,
		verifyReq:  false,
		tunnel: sql.NullString{
			String: "",
			Valid:  false,
		},
	}
	for _, option := range options {
		switch option {
		case "agent-forwarding":
			ak.agentForwarding = true
		case "cert-authority":
			ak.IsCA = true
		case "no-agent-forwarding":
			ak.agentForwarding = false
		case "no-port-forwarding":
			ak.PortForwarding = false
		case "no-pty":
			ak.pty = false
		case "no-user-rc":
			ak.UserRC = false
		case "no-x11-forwarding":
			ak.X11Forwarding = false
		case "port-forwarding":
			ak.PortForwarding = true
		case "pty":
			ak.pty = true
		case "no-touch-required":
			ak.noTouchReq = true
		case "verify-required":
			ak.verifyReq = true
		case "user-rc":
			ak.UserRC = true
		case "X11-forwarding":
			ak.X11Forwarding = true
		case "restrict":
			ak.agentForwarding = false
			ak.PortForwarding = false
			ak.pty = false
			ak.UserRC = false
			ak.X11Forwarding = false
		default:
			parts := strings.SplitN(option, "=", 2)
			if len(parts) == 2 {
				command := parts[0]
				value := parts[1]
				if value[0] == '"' && value[len(value)-1] == '"' {
					value = value[1 : len(value)-1] // remove quotes if present
				}
				switch command {
				case "command":
					ak.command.Valid = true
					ak.command.String = value
				case "environment":
					envParts := strings.SplitN(value, "=", 2)
					if len(envParts) == 2 {
						ak.environment[envParts[0]] = envParts[1]
					} else {
						return nil, fmt.Errorf("invalid environment option: %s", value)
					}
				case "expiry-time":
					timespec, err := ParseSSHTimespec(value)
					if err != nil {
						return nil, fmt.Errorf("invalid expiry-time: %s", value)
					}
					ak.expiryTime.Valid = true
					ak.expiryTime.Time = timespec
				case "from":
					parts := strings.Split(value, ",") // ssh_config man page doesn't specify escaping rules, so we'll just split on commas
					for _, part := range parts {
						pattern, err := NewPattern(part)
						if err != nil {
							return nil, fmt.Errorf("invalid from option: %s", value)
						}
						ak.from = append(ak.from, pattern)

					}
				case "permit-listen":
					ak.permitListen.Valid = true
					ak.permitListen.String = value
				case "permit-open":
					ak.permitOpen.Valid = true
					ak.permitOpen.String = value
				case "principal":
					parts := strings.Split(value, ",") // ssh_config man page doesn't specify escaping rules, so we'll just split on commas
					ak.Principals = append(ak.Principals, parts...)
				case "tunnel":
					ak.tunnel.Valid = true
					ak.tunnel.String = value
				default:
					return nil, fmt.Errorf("unknown option: %s", option)
				}
			} else {
				return nil, fmt.Errorf("unknown option: %s", option)
			}
		}
	}
	return ak, nil
}
