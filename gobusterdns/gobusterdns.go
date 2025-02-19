package gobusterdns

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"yBuster/libgobuster"

	"github.com/google/uuid"
)

// GobusterDNS is the main type to implement the interface
type GobusterDNS struct{}

// Setup is the setup implementation of gobusterdns
func (d GobusterDNS) Setup(g *libgobuster.Gobuster) error {
	// Resolve a subdomain sthat probably shouldn't exist
	guid := uuid.New()
	wildcardIps, err := g.DNSLookup(fmt.Sprintf("%s.%s", guid, g.Opts.URL))
	if err == nil {
		g.IsWildcard = true
		g.WildcardIps.AddRange(wildcardIps)
		log.Printf("[-] Wildcard DNS found. IP address(es): %s", g.WildcardIps.Stringify())
		if !g.Opts.WildcardForced {
			return fmt.Errorf("To force processing of Wildcard DNS, specify the '-fw' switch.")
		}
	}

	if !g.Opts.Quiet {
		// Provide a warning if the base domain doesn't resolve (in case of typo)
		_, err = g.DNSLookup(g.Opts.URL)
		if err != nil {
			// Not an error, just a warning. Eg. `yp.to` doesn't resolve, but `cr.py.to` does!
			log.Printf("[-] Unable to validate base domain: %s", g.Opts.URL)
		}
	}

	return nil
}

// Process is the process implementation of gobusterdns
func (d GobusterDNS) Process(g *libgobuster.Gobuster, busterTarget *libgobuster.BusterTarget) ([]libgobuster.Result, error) {
	subdomain := fmt.Sprintf("%s.%s", busterTarget.Target, g.Opts.URL)
	ips, err := g.DNSLookup(subdomain)
	var ret []libgobuster.Result
	if err == nil {
		if !g.IsWildcard || !g.WildcardIps.ContainsAny(ips) {
			result := libgobuster.Result{
				Entity: subdomain,
			}
			if g.Opts.ShowIPs {
				result.Extra = strings.Join(ips, ", ")
			} else if g.Opts.ShowCNAME {
				cname, err := g.DNSLookupCname(subdomain)
				if err == nil {
					result.Extra = cname
				}
			}
			ret = append(ret, result)
		}
	} else if g.Opts.Verbose {
		ret = append(ret, libgobuster.Result{
			Entity: subdomain,
			Status: 404,
		})
	}
	return ret, nil
}

// ResultToString is the to string implementation of gobusterdns
func (d GobusterDNS) ResultToString(g *libgobuster.Gobuster, r *libgobuster.Result) (*string, *string, int, error) {
	buf := &bytes.Buffer{}

	if r.Status == 404 {
		if _, err := fmt.Fprintf(buf, "Missing: %s\n", r.Entity); err != nil {
			return nil, nil, 0, err
		}
	} else if g.Opts.ShowIPs {
		if _, err := fmt.Fprintf(buf, "Found: %s [%s]\n", r.Entity, r.Extra); err != nil {
			return nil, nil, 0, err
		}
	} else if g.Opts.ShowCNAME {
		if _, err := fmt.Fprintf(buf, "Found: %s [%s]\n", r.Entity, r.Extra); err != nil {
			return nil, nil, 0, err
		}
	} else {
		if _, err := fmt.Fprintf(buf, "Found: %s\n", r.Entity); err != nil {
			return nil, nil, 0, err
		}
	}

	s := buf.String()
	return &s, nil, r.Status, nil
}
