package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nrdcg/dnspod-go"
)

const (
	domainTpl = `
resource "dnspod_domain" "{{ .Name | normalizeDomain }}" {
	domain = "{{ .Name }}"
}
`
	recordsTpl = `
{{- range .Records}}
resource "dnspod_record" "{{ . | uniqueRecordName $.Domain }}" {
  domain_id       = dnspod_domain.{{ $.Domain.Name | normalizeDomain }}.id
  sub_domain      = "{{ .Name }}"
  record_type     = "{{ .Type }}"
  record_line     = "{{ .Line }}"
  value           = "{{ .Value }}"
  mx              = "{{ .MX }}"
  ttl             = "{{ .TTL }}"
  status          = "{{ .Status }}"
{{- if .Weight }}
  weight          = {{ .Weight }}
{{- end}}
}
{{ end}}
`
)

func normalizeDomain(a string) string {
	a = strings.ReplaceAll(a, ".", "_")
	return a
}

func normalizeRR(a string) string {
	a = strings.ReplaceAll(a, ".", "_")
	a = strings.ReplaceAll(a, "*", "wildcard")
	a = strings.ReplaceAll(a, "@", "at")
	return a
}

func uniqueRecordName(d *dnspod.Domain, r *dnspod.Record) string {
	return fmt.Sprintf("%s_%s_%s", normalizeDomain(d.Name), normalizeRR(r.Name), r.ID)
}

var funcMap = template.FuncMap{
	"normalizeDomain":  normalizeDomain,
	"uniqueRecordName": uniqueRecordName,
}

func generate(client *dnspod.Client, d *dnspod.Domain) {
	stateFileName := filepath.Join(optOutput, fmt.Sprintf("dnspod_%s.tf", normalizeDomain(d.Name)))
	tf, err := os.OpenFile(stateFileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer tf.Close()
	importFileName := filepath.Join(optOutput, fmt.Sprintf("dnspod_%s.sh", normalizeDomain(d.Name)))
	importF, err := os.OpenFile(importFileName, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer importF.Close()
	fmt.Fprintf(importF, "set -o errexit\n")
	records, _, err := client.Records.List(string(d.ID), "")
	if err != nil {
		log.Fatal(err)
	}
	t := template.Must(template.New("domainTpl").Funcs(funcMap).Parse(domainTpl))
	err = t.Execute(tf, d)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(importF, "terraform import dnspod_domain.%s %s\n", normalizeDomain(d.Name), d.ID)
	t = template.Must(template.New("recordsTpl").Funcs(funcMap).Parse(recordsTpl))
	err = t.Execute(tf, struct {
		Domain  *dnspod.Domain
		Records []dnspod.Record
	}{
		Domain:  d,
		Records: records,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range records {
		fmt.Fprintf(importF, "terraform import dnspod_record.%s %s:%s\n", uniqueRecordName(d, &item), d.ID, item.ID)
	}
}

var (
	optDomain string
	optOutput string
)

func init() {
	flag.StringVar(&optDomain, "domain", "", "domain name")
	flag.StringVar(&optOutput, "output", "", "output directory")
}

func main() {
	flag.Parse()
	loginToken := os.Getenv("DNSPOD_LOGIN_TOKEN")
	if loginToken == "" {
		log.Fatal("DNSPOD_LOGIN_TOKEN is not set")
	}
	params := dnspod.CommonParams{LoginToken: loginToken, Format: "json"}
	client := dnspod.NewClient(params)
	domains, _, err := client.Domains.List()
	if err != nil {
		log.Fatalln(err)
	}
	for _, domain := range domains {
		log.Printf("Domain: %s (id: %s)\n", domain.Name, domain.ID)
		if optDomain != "" && optDomain != domain.Name {
			log.Printf("domain %s is ignored", domain.Name)
			continue
		}
		generate(client, &domain)
	}
}
