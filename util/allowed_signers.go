package util

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
	"time"
)

type AllowedSigner struct {
	Key         ssh.PublicKey
	Principals  []*Pattern
	Namespaces  []*Pattern
	IsCA        bool
	ValidAfter  *time.Time
	ValidBefore *time.Time
}

// ParseAllowedSigners parses a list of AllowedSigners from a byte slice.
func ParseAllowedSigners(in []byte) ([]AllowedSigner, error) {
	lines := bytes.Split(in, []byte("\n"))
	var signers []AllowedSigner
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		signer, err := parseAllowedSigner(line)
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

func parseAllowedSigner(line []byte) (AllowedSigner, error) {
	fields := bytes.Fields(line) // this wouldn't work if some option (e.g. namespace) would contain a space,
	//                              but is that allowed in the spec?
	if len(fields) < 3 {
		return AllowedSigner{}, fmt.Errorf("invalid allowed signer line: %q", line)
	}
	principals := strings.Split(string(fields[0]), ",")
	options := strings.Split(string(fields[1]), ",")
	keyType := string(fields[2])
	key := string(fields[3])
	var as AllowedSigner
	for _, principal := range principals {
		p, err := NewPattern(principal)
		if err != nil {
			return AllowedSigner{}, fmt.Errorf("invalid principal pattern: %s", principal)
		}
		as.Principals = append(as.Principals, p)
	}
	for _, option := range options {
		switch {
		case option == "cert-authority":
			as.IsCA = true
		case strings.HasPrefix(option, "namespaces="):
			ns := strings.TrimPrefix(option, "namespaces=")
			nsList := strings.Split(ns, ",")
			for _, n := range nsList {
				p, err := NewPattern(n)
				if err != nil {
					return AllowedSigner{}, fmt.Errorf("invalid namespace pattern: %s", n)
				}
				as.Namespaces = append(as.Namespaces, p)
			}
		case strings.HasPrefix(option, "valid-after="):
			timestamp := strings.TrimPrefix(option, "valid-after=")
			t, err := ParseSSHTimespec(timestamp)
			if err != nil {
				return AllowedSigner{}, fmt.Errorf("invalid valid-after timestamp: %s", timestamp)
			}
			as.ValidAfter = &t
		case strings.HasPrefix(option, "valid-before="):
			timestamp := strings.TrimPrefix(option, "valid-before=")
			t, err := ParseSSHTimespec(timestamp)
			if err != nil {
				return AllowedSigner{}, fmt.Errorf("invalid valid-before timestamp: %s", timestamp)
			}
			as.ValidBefore = &t
		default:
			return AllowedSigner{}, fmt.Errorf("unknown option: %s", option)
		}
	}
	authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key + " " + keyType))
	if err != nil {
		return AllowedSigner{}, fmt.Errorf("error parsing authorized key: %w", err)
	}
	as.Key = authorizedKey
	return as, nil
}
