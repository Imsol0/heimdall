package types

import (
	"net"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
)

type CtLog struct {
	Id     string
	Name   string
	Client *client.LogClient
}

type EntryTask struct {
	Entries *ct.GetEntriesResponse
	Index   int64
}

type CertificateInfo struct {
	OriginIP         string   `json:"originip"`
	Organization     []string `json:"org"`
	OrganizationUnit []string `json:"orgunit"`
	CommonName       string   `json:"commonName"`
	SAN              []string `json:"san"`
	Domains          []string `json:"domains"`
	Emails           []string `json:"emails"`
	IPAddrs          []net.IP `json:"ips"`
}
