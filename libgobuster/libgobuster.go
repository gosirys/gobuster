package libgobuster

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// VERSION contains the current gobuster version
	VERSION = "2.0.1"
)

// SetupFunc is the "setup" function prototype for implementations
type SetupFunc func(*Gobuster) error

// ProcessFunc is the "process" function prototype for implementations
type ProcessFunc func(*Gobuster, string) ([]Result, error)

// ResultToStringFunc is the "to string" function prototype for implementations
type ResultToStringFunc func(*Gobuster, *Result) (*string, *string, int, error)

// Gobuster is the main object when creating a new run
type Gobuster struct {
	Opts                          *Options
	HTTP                          *httpClient
	WildcardIps                   stringSet
	context                       context.Context
	requestsExpected              int
	requestsIssued                int
	mu                            *sync.RWMutex
	plugin                        GobusterPlugin
	IsWildcard                    bool
	IsWildcardFileByContentLength bool
	IsWildcardDirByContentLength  bool
	WildcardFileContentLength     int
	WildcardDirContentLength      int
	IsWildcardFileByTitle         bool
	IsWildcardDirByTitle          bool
	WildcardFileTitle             string
	WildcardDirTitle              string
	WildcardStatusCode            *int
	resultChan                    chan Result
	errorChan                     chan error
	errorCount                    int
	waybackParsed                 string
}

// BusterTarget is target is the entity to be processed
type BusterTarget struct {
	IsURL  bool
	Target string
}

// ParsedURL is used to store parsed urls
type ParsedURL struct {
	Host  string
	Path  string
	Query url.Values
	URL   string
}

// GobusterPlugin is an interface which plugins must implement
type GobusterPlugin interface {
	Setup(*Gobuster) error
	Process(*Gobuster, *BusterTarget) ([]Result, error)
	ResultToString(*Gobuster, *Result) (*string, *string, int, error)
}

// NewGobuster returns a new Gobuster object
func NewGobuster(c context.Context, opts *Options, plugin GobusterPlugin) (*Gobuster, error) {
	// validate given options
	multiErr := opts.validate()
	if multiErr != nil {
		return nil, multiErr
	}

	var g Gobuster
	g.WildcardIps = newStringSet()
	g.context = c
	g.Opts = opts
	h, err := newHTTPClient(c, opts)
	if err != nil {
		return nil, err
	}
	g.HTTP = h

	g.plugin = plugin
	g.mu = new(sync.RWMutex)

	g.resultChan = make(chan Result)
	g.errorChan = make(chan error)

	return &g, nil
}

// Results returns a channel of Results
func (g *Gobuster) Results() <-chan Result {
	return g.resultChan
}

// Errors returns a channel of errors
func (g *Gobuster) Errors() <-chan error {
	return g.errorChan
}

func (g *Gobuster) incrementRequests() {
	g.mu.Lock()
	g.requestsIssued++
	g.mu.Unlock()
}

// DecrementRequests decrements the requests issued
func (g *Gobuster) DecrementRequests() {
	g.mu.Lock()
	if g.requestsIssued > 0 {
		g.requestsIssued--
	}
	g.mu.Unlock()
}

// IncrementErrorCount increments the error count
func (g *Gobuster) IncrementErrorCount() {
	g.mu.Lock()
	g.errorCount++
	g.mu.Unlock()
}

// PrintProgress outputs the current wordlist progress to stderr
func (g *Gobuster) PrintProgress() {
	if !g.Opts.Quiet && !g.Opts.NoProgress {
		g.mu.RLock()
		if g.Opts.Wordlist == "-" {
			fmt.Fprintf(os.Stderr, "\rProgress: %d", g.requestsIssued)
			// only print status if we already read in the wordlist
		} else if g.requestsExpected > 0 {
			if !g.Opts.Verbose {
				fmt.Fprintf(os.Stderr, "\rProgress: %d / %d (%3.2f%%)  |  Errors:  %d / %d (%3.2f%%)\r", g.requestsIssued, g.requestsExpected, float32(g.requestsIssued)*100.0/float32(g.requestsExpected), g.errorCount, g.requestsExpected, float32(g.errorCount)*100.0/float32(g.requestsExpected))
			} else {
				fmt.Fprintf(os.Stderr, "\rProgress: %d / %d (%3.2f%%)\r", g.requestsIssued, g.requestsExpected, float32(g.requestsIssued)*100.0/float32(g.requestsExpected))
			}
		}
		g.mu.RUnlock()
	}
}

// ClearProgress removes the last status line from stderr
func (g *Gobuster) ClearProgress() {
	fmt.Fprint(os.Stderr, resetTerminal())
}

// GetRequest issues a GET request to the target and returns
// the status code, length and an error
func (g *Gobuster) GetRequest(url string) (*int, *int64, *string, *string, error) {
	return g.HTTP.makeRequest(url, g.Opts.Cookies)
}

// DNSLookup looks up a domain via system default DNS servers
func (g *Gobuster) DNSLookup(domain string) ([]string, error) {
	return net.LookupHost(domain)
}

// DNSLookupCname looks up a CNAME record via system default DNS servers
func (g *Gobuster) DNSLookupCname(domain string) (string, error) {
	return net.LookupCNAME(domain)
}

func (g *Gobuster) worker(wordChan <-chan *BusterTarget, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-g.context.Done():
			return
		case busterTarget, ok := <-wordChan:
			// worker finished
			if !ok {
				return
			}
			g.incrementRequests()
			// Mode-specific processing
			res, err := g.plugin.Process(g, busterTarget)
			if err != nil {
				// do not exit and continue
				g.errorChan <- err
				continue
			} else {
				for _, r := range res {
					g.resultChan <- r
				}
			}
		}
	}
}

func (g *Gobuster) getWordlist() (*bufio.Scanner, error) {
	if g.Opts.Wordlist == "-" {
		// Read directly from stdin
		return bufio.NewScanner(os.Stdin), nil
	}
	// Pull content from the wordlist
	wordlist, err := os.Open(g.Opts.Wordlist)
	if err != nil {
		return nil, fmt.Errorf("failed to open wordlist: %v", err)
	}

	wordExtensionScanner := bufio.NewScanner(wordlist)
	wordExtensionCount := 0
	lines := 0
	for wordExtensionScanner.Scan() {
		word := strings.TrimSpace(wordExtensionScanner.Text())
		if word == "" {
			continue
		}
		lines++
		if strings.Contains(word, "%EXT%") {
			wordExtensionCount++
		}
	}
	if serr := wordExtensionScanner.Err(); serr != nil {
		return nil, fmt.Errorf("failed to scan word list for extensions: %v", serr)
	}

	g.requestsIssued = 0
	if g.Opts.BlankExtension {
		g.requestsExpected = lines + wordExtensionCount*len(g.Opts.ExtensionsParsed.Set)
	} else {
		g.requestsExpected = lines + wordExtensionCount*len(g.Opts.ExtensionsParsed.Set) - wordExtensionCount
	}

	// rewind wordlist
	_, err = wordlist.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to rewind wordlist: %v", err)
	}
	return bufio.NewScanner(wordlist), nil
}

func (g *Gobuster) getWaybackUrls() (*bufio.Scanner, error) {
	err := g.parseWaybackUrls()
	if err != nil {
		return nil, fmt.Errorf("failed to parse wayback urls: %v", err)
	}

	waybackUrls, err := os.Open(g.waybackParsed)
	if err != nil {
		return nil, fmt.Errorf("failed to open parsed wayback: %v", err)
	}

	scanner := bufio.NewScanner(waybackUrls)
	lines := 0
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word == "" {
			continue
		}
		lines++
	}
	if serr := scanner.Err(); serr != nil {
		return nil, fmt.Errorf("failed to scan parsed way back: %v", serr)
	}

	g.requestsExpected = lines
	g.requestsIssued = 0

	// rewind waybackurls
	_, err = waybackUrls.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to rewind parsed wayback: %v", err)
	}
	return bufio.NewScanner(waybackUrls), nil
}

func (g *Gobuster) parseWaybackUrls() error {

	// log.Printf("fucken %s",g.Opts.OutputFolder)

	waybackUrls, err := os.Open(g.Opts.WaybackUrls)
	if err != nil {
		return fmt.Errorf("failed to open wayback urls: %v", err)
	}

	// rewind waybackurls
	_, err = waybackUrls.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to rewind wayback urls: %v", err)
	}

	scanner := bufio.NewScanner(waybackUrls)
	var waybackLines []string
	for scanner.Scan() {
		waybackLines = append(waybackLines, scanner.Text())
	}

	log.Printf("Loading waybackurls file -> %s - Loaded %d", g.Opts.WaybackUrls, len(waybackLines))

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan wayback urls: %v", err)
	}

	sort.Strings(waybackLines)

	

	var parsedUrls []ParsedURL
	for _, line := range waybackLines {
		u, err := url.Parse(line)
		host := ""
		path := ""
		query := new(url.Values)
		if err != nil {
			host = line
		} else {
			host = u.Host
			path = u.Path
			*query = u.Query()
		}
		// log.Printf("hostz: %d", host)BusterTarget



		rgx := regexp.MustCompile(`(?i)[^/]+\.(jpg|jpeg|woff|woff2|ico|css|eot|pdf|ttf|gif|doc|docx|xls|xlsx|svg|csv|mp3|mp4|wma|ppt|png|pptx|swf).*`)
		cleanPath := rgx.ReplaceAllString(path, "")
		cleanURL := rgx.ReplaceAllString(line, "")

		parsedUrls = append(parsedUrls, ParsedURL{
			Host:  host,
			Path:  cleanPath,
			Query: *query,
			URL:   cleanURL,
		})
	}

	var uniqueParsedUrls []ParsedURL
	for _, parsedURL := range parsedUrls {
		if len(uniqueParsedUrls) == 0 {
			uniqueParsedUrls = append(uniqueParsedUrls, parsedURL)
			continue
		}
		isURLMatching := false
		for _, value := range uniqueParsedUrls {
			isQueryMatching := false
			if value.Host == parsedURL.Host && value.Path == parsedURL.Path {
				if len(parsedURL.Query) > 0 && len(value.Query) == len(parsedURL.Query) {
					for parsedURLQueryKey := range parsedURL.Query {
						if _, ok := value.Query[parsedURLQueryKey]; ok {
							isQueryMatching = true
						} else {
							isQueryMatching = false
							break
						}
					}
				} else if len(value.Query) == 0 && len(parsedURL.Query) == 0 {
					isQueryMatching = true
				}
			}

			if isQueryMatching {
				isURLMatching = true
				break
			}
		}
		if !isURLMatching {
			uniqueParsedUrls = append(uniqueParsedUrls, parsedURL)
		}
	}

	var uniqueUrls []string
	for _, value := range uniqueParsedUrls {
		uniqueUrls = append(uniqueUrls, value.URL)
	}

	log.Printf("Total unique URLs from wayback file parsed: %d", len(uniqueUrls))

	filenameTimeStamp := int32(time.Now().Unix())
	parsedMainURL, _ := url.Parse(g.Opts.URL)
	sanitizedHost := strings.ReplaceAll(parsedMainURL.Host, ".", "_")
	sanitizedHost = strings.ReplaceAll(sanitizedHost, ":", "_")
	sanitizedPath := ""
	if parsedMainURL.Path != "/" {
		sanitizedPath = strings.TrimSuffix(parsedMainURL.Path, "/")
		sanitizedPath = strings.ReplaceAll(sanitizedPath, "/", "_")
	}

	g.waybackParsed = fmt.Sprintf(g.Opts.OutputFolder + "/output_waybackurls/waybackurls_parsed_%d_%s_%s%s.txt", filenameTimeStamp, parsedMainURL.Scheme, sanitizedHost, sanitizedPath)
	waybackUrlsParsed, err := os.Create(g.waybackParsed)
	if err != nil {
		return fmt.Errorf("failed to create wayback parsed: %v", err)
	}
	defer waybackUrlsParsed.Close()

	writer := bufio.NewWriter(waybackUrlsParsed)
	for _, line := range uniqueUrls {
		fmt.Fprintln(writer, line)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to write wayback urls: %v", err)
	}

	return nil
}

// Start the busting of the website with the given
// set of settings from the command line.
func (g *Gobuster) Start() error {
	if err := g.plugin.Setup(g); err != nil {
		return err
	}

	var workerGroup sync.WaitGroup
	workerGroup.Add(g.Opts.Threads)

	wordChan := make(chan *BusterTarget, g.Opts.Threads)

	// Create goroutines for each of the number of threads
	// specified.
	for i := 0; i < g.Opts.Threads; i++ {
		go g.worker(wordChan, &workerGroup)
	}

	if g.Opts.WaybackUrls != "" {
		waybackScanner, err := g.getWaybackUrls()
		if err != nil {
			return err
		}

		log.Printf("Starting requesting waybackurls..")

	WaybackScan:
		for waybackScanner.Scan() {
			select {
			case <-g.context.Done():
				break WaybackScan
			default:
				url := strings.TrimSpace(waybackScanner.Text())
				// Skip "comment" (starts with #), as well as empty lines
				if !strings.HasPrefix(url, "#") && len(url) > 0 {
					busterTarget := &BusterTarget{
						IsURL:  true,
						Target: url,
					}
					wordChan <- busterTarget
				}
			}
		}

		time.Sleep(5 * time.Second)
		log.Printf("waybackurls parsing and requesting done.")
	}

	log.Printf("Starting dictionary based brute-force..")

	wordScanner, err := g.getWordlist()
	if err != nil {
		return err
	}

WordScan:
	for wordScanner.Scan() {
		select {
		case <-g.context.Done():
			break WordScan
		default:
			word := strings.TrimSpace(wordScanner.Text())
			// Skip "comment" (starts with #), as well as empty lines
			if !strings.HasPrefix(word, "#") && len(word) > 0 {
				if strings.Contains(word, "%EXT%") {
					if g.Opts.BlankExtension {
						sanitizedWord := strings.ReplaceAll(word, ".%EXT%", "")
						busterTarget := &BusterTarget{
							IsURL:  false,
							Target: sanitizedWord,
						}
						wordChan <- busterTarget
					}
					for ext := range g.Opts.ExtensionsParsed.Set {
						wordWithExt := strings.ReplaceAll(word, "%EXT%", ext)
						busterTarget := &BusterTarget{
							IsURL:  false,
							Target: wordWithExt,
						}
						wordChan <- busterTarget
					}
				} else {
					busterTarget := &BusterTarget{
						IsURL:  false,
						Target: word,
					}
					wordChan <- busterTarget
				}
			}
		}
	}

	close(wordChan)
	workerGroup.Wait()
	close(g.resultChan)
	close(g.errorChan)
	return nil
}

// GetConfigString returns the current config as a printable string
func (g *Gobuster) GetConfigString() (string, error) {
	buf := &bytes.Buffer{}
	o := g.Opts
	if _, err := fmt.Fprintf(buf, "[+] Mode                  : %s\n", o.Mode); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(buf, "[+] Url/Domain            : %s\n", o.URL); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(buf, "[+] Threads               : %d\n", o.Threads); err != nil {
		return "", err
	}

	wordlist := "stdin (pipe)"
	if o.Wordlist != "-" {
		wordlist = o.Wordlist
	}
	if _, err := fmt.Fprintf(buf, "[+] Wordlist              : %s\n", wordlist); err != nil {
		return "", err
	}

	if o.Mode == ModeDir {
		if o.ExcludedStatusCodes != "" {
			if _, err := fmt.Fprintf(buf, "[+] Excluded status codes : %s\n", o.ExcludedStatusCodesParsed.Stringify()); err != nil {
				return "", err
			}
		}

		if o.Proxy != "" {
			if _, err := fmt.Fprintf(buf, "[+] Proxy                 : %s\n", o.Proxy); err != nil {
				return "", err
			}
		}

		if o.Cookies != "" {
			if _, err := fmt.Fprintf(buf, "[+] Cookies               : %s\n", o.Cookies); err != nil {
				return "", err
			}
		}

		if o.UserAgent != "" {
			if _, err := fmt.Fprintf(buf, "[+] User Agent            : %s\n", o.UserAgent); err != nil {
				return "", err
			}
		}

		if o.IncludeLength {
			if _, err := fmt.Fprintf(buf, "[+] Show length           : true\n"); err != nil {
				return "", err
			}
		}

		if o.Username != "" {
			if _, err := fmt.Fprintf(buf, "[+] Auth User             : %s\n", o.Username); err != nil {
				return "", err
			}
		}

		if len(o.Extensions) > 0 {
			if _, err := fmt.Fprintf(buf, "[+] Extensions            : %s\n", o.ExtensionsParsed.Stringify()); err != nil {
				return "", err
			}
		}

		if o.UseSlash {
			if _, err := fmt.Fprintf(buf, "[+] Add Slash             : true\n"); err != nil {
				return "", err
			}
		}

		if o.FollowRedirect {
			if _, err := fmt.Fprintf(buf, "[+] Follow Redir          : true\n"); err != nil {
				return "", err
			}
		}

		if o.Expanded {
			if _, err := fmt.Fprintf(buf, "[+] Expanded              : true\n"); err != nil {
				return "", err
			}
		}

		if o.NoStatus {
			if _, err := fmt.Fprintf(buf, "[+] No status             : true\n"); err != nil {
				return "", err
			}
		}

		if o.Verbose {
			if _, err := fmt.Fprintf(buf, "[+] Verbose               : true\n"); err != nil {
				return "", err
			}
		}

		if _, err := fmt.Fprintf(buf, "[+] Timeout               : %s\n", o.Timeout.String()); err != nil {
			return "", err
		}

		if o.WaybackUrls != "" {
			if _, err := fmt.Fprintf(buf, "[+] Wayback urls          : %s\n", o.WaybackUrls); err != nil {
				return "", err
			}
		}

		if o.RandomAgent != "" {
			if _, err := fmt.Fprintf(buf, "[+] Random agent          : %s\n", o.RandomAgent); err != nil {
				return "", err
			}
		}

		if o.TargetUrls != "" {
			if _, err := fmt.Fprintf(buf, "[+] Target urls           : %s\n", o.TargetUrls); err != nil {
				return "", err
			}
		}

		if o.ExcludeString != "" {
			if _, err := fmt.Fprintf(buf, "[+] Exclude string         : %s\n", o.ExcludeString); err != nil {
				return "", err
			}
		}

		if o.BlankExtension {
			if _, err := fmt.Fprintf(buf, "[+] Blank extension       : true\n"); err != nil {
				return "", err
			}
		}


		if o.OutputFolder != "" {
			if _, err := fmt.Fprintf(buf, "[+] Output folder         : %s\n", o.OutputFolder); err != nil {
				return "", err
			}
		}
	}

	return strings.TrimSpace(buf.String()), nil
}
