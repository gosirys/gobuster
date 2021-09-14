package libgobuster

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
)

const (
	// ModeDir represents -m dir
	ModeDir = "dir"
	// ModeDNS represents -m dns
	ModeDNS = "dns"
)

// Options helds all options that can be passed to libgobuster
type Options struct {
	Extensions                string
	ExtensionsParsed          stringSet
	Mode                      string
	OutputFilename			  string
	OutputFolder			  string
	Password                  string
	ExcludedStatusCodes       string
	ExcludedStatusCodesParsed intSet
	Threads                   int
	URL                       string
	UserAgent                 string
	Username                  string
	Wordlist                  string
	Proxy                     string
	Cookies                   string
	Timeout                   time.Duration
	FollowRedirect            bool
	IncludeLength             bool
	NoStatus                  bool
	NoProgress                bool
	Expanded                  bool
	Quiet                     bool
	ShowIPs                   bool
	ShowCNAME                 bool
	InsecureSSL               bool
	WildcardForced            bool
	Verbose                   bool
	UseSlash                  bool
	WaybackUrls               string
	TargetUrls                string
	RandomAgent               string
	RandomAgentParsed         []string
	ExcludeString             string
	BlankExtension            bool
}

// NewOptions returns a new initialized Options object
func NewOptions() *Options {
	return &Options{
		ExcludedStatusCodesParsed: newIntSet(),
		ExtensionsParsed:          newStringSet(),
	}
}

// Validate validates the given options
func (opt *Options) validate() *multierror.Error {
	var errorList *multierror.Error

	if strings.ToLower(opt.Mode) != ModeDir && strings.ToLower(opt.Mode) != ModeDNS {
		errorList = multierror.Append(errorList, fmt.Errorf("Mode (-m): Invalid value: %s", opt.Mode))
	}

	if opt.Threads < 0 {
		errorList = multierror.Append(errorList, fmt.Errorf("Threads (-t): Invalid value: %d", opt.Threads))
	}

	if opt.Wordlist == "" {
		errorList = multierror.Append(errorList, fmt.Errorf("WordList (-w): Must be specified (use `-w -` for stdin)"))
	} else if opt.Wordlist == "-" {
		// STDIN
	} else if _, err := os.Stat(opt.Wordlist); os.IsNotExist(err) {
		errorList = multierror.Append(errorList, fmt.Errorf("Wordlist (-w): File does not exist: %s", opt.Wordlist))
	}

	if opt.URL == "" {
		errorList = multierror.Append(errorList, fmt.Errorf("Url/Domain (-u): Must be specified: %s",opt.URL))
	}

	if opt.OutputFolder == "" {
		errorList = multierror.Append(errorList, fmt.Errorf("Output folder (-of): Must be specified: %s",opt.OutputFolder))
	}


	if opt.ExcludedStatusCodes != "" {
		if err := opt.parseStatusCodes(); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	if opt.Extensions != "" {
		if err := opt.parseExtensions(); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	if opt.Mode == ModeDir {
		if !strings.HasSuffix(opt.URL, "/") {
			opt.URL = fmt.Sprintf("%s/", opt.URL)
		}

		if err := opt.validateDirMode(); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	if opt.WaybackUrls != "" {
		if _, err := os.Stat(opt.WaybackUrls); os.IsNotExist(err) {
			errorList = multierror.Append(errorList, fmt.Errorf("Wayback urls (-waybackurls): File does not exist: %s", opt.WaybackUrls))
		}
	}

	if opt.RandomAgent != "" {
		if _, err := os.Stat(opt.RandomAgent); os.IsNotExist(err) {
			errorList = multierror.Append(errorList, fmt.Errorf("Random agent (-random-agent): File does not exist: %s", opt.RandomAgent))
		} else {
			if err := opt.parseRandomAgents(); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}
	}

	if opt.TargetUrls != "" {
		if _, err := os.Stat(opt.TargetUrls); os.IsNotExist(err) {
			errorList = multierror.Append(errorList, fmt.Errorf("Target urls (-target-urls): File does not exist: %s", opt.TargetUrls))
		}
	}

	return errorList
}

// ParseExtensions parses the extensions provided as a comma seperated list
func (opt *Options) parseExtensions() error {
	if opt.Extensions == "" {
		return fmt.Errorf("invalid extension string provided")
	}

	exts := strings.Split(opt.Extensions, ",")
	for _, e := range exts {
		e = strings.TrimSpace(e)
		// remove leading . from extensions
		opt.ExtensionsParsed.Add(strings.TrimPrefix(e, "."))
	}
	return nil
}

// ParseStatusCodes parses the status codes provided as a comma seperated list
func (opt *Options) parseStatusCodes() error {
	if opt.ExcludedStatusCodes == "" {
		return fmt.Errorf("invalid status code string provided")
	}

	for _, c := range strings.Split(opt.ExcludedStatusCodes, ",") {
		c = strings.TrimSpace(c)
		i, err := strconv.Atoi(c)
		if err != nil {
			return fmt.Errorf("invalid status code given: %s", c)
		}
		opt.ExcludedStatusCodesParsed.Add(i)
	}
	return nil
}

func (opt *Options) parseRandomAgents() error {
	randomAgents, err := os.Open(opt.RandomAgent)
	if err != nil {
		return fmt.Errorf("failed to open random agents: %v", err)
	}

	// rewind random agents
	_, err = randomAgents.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to rewind random agents: %v", err)
	}

	scanner := bufio.NewScanner(randomAgents)
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "#") && len(scanner.Text()) > 0 {
			opt.RandomAgentParsed = append(opt.RandomAgentParsed, scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan random agents: %v", err)
	}

	return nil
}

func (opt *Options) validateDirMode() error {
	// bail out if we are not in dir mode
	if opt.Mode != ModeDir {
		return nil
	}
	if !strings.HasPrefix(opt.URL, "http") {
		// check to see if a port was specified
		re := regexp.MustCompile(`^[^/]+:(\d+)`)
		match := re.FindStringSubmatch(opt.URL)

		if len(match) < 2 {
			// no port, default to http on 80
			opt.URL = fmt.Sprintf("http://%s", opt.URL)
		} else {
			port, err := strconv.Atoi(match[1])
			if err != nil || (port != 80 && port != 443) {
				return fmt.Errorf("url scheme not specified")
			} else if port == 80 {
				opt.URL = fmt.Sprintf("http://%s", opt.URL)
			} else {
				opt.URL = fmt.Sprintf("https://%s", opt.URL)
			}
		}
	}

	if opt.Username != "" && opt.Password == "" {
		return fmt.Errorf("username was provided but password is missing")
	}

	return nil
}
