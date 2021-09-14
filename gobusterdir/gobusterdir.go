package gobusterdir

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"time"

	"yBuster/libgobuster"

	"github.com/google/uuid"
)

// GobusterDir is the main type to implement the interface
type GobusterDir struct{}

// Setup is the setup implementation of gobusterdir
func (d GobusterDir) Setup(g *libgobuster.Gobuster) error {
	_, _, _, _, err := g.GetRequest(g.Opts.URL)
	if err != nil {
		return fmt.Errorf("unable to connect to %s: %v", g.Opts.URL, err)
	}

	r := regexp.MustCompile(`(?s).*<title>(?P<Title>.*)<\/title>.*`)
	g.WildcardStatusCode = new(int)

	uuidFile16 := strings.ReplaceAll(uuid.New().String(), "-", "")[0:16]
	urlFile16 := fmt.Sprintf("%s%s", g.Opts.URL, uuidFile16)
	wildcardRespFile16, _, wildcardContentFile16, _, errFile16 := g.GetRequest(urlFile16)
	if errFile16 != nil {
		return errFile16
	}
	cleanWildcardContentFile16 := strings.ReplaceAll(*wildcardContentFile16, urlFile16, "")
	rsFile16 := r.FindStringSubmatch(*wildcardContentFile16)
	cleanTitleFile16 := ""
	if len(rsFile16) > 0 {
		cleanTitleFile16 = strings.TrimSpace(rsFile16[1])
	}

	uuidFile8 := strings.ReplaceAll(uuid.New().String(), "-", "")[0:8]
	urlFile8 := fmt.Sprintf("%s%s", g.Opts.URL, uuidFile8)
	wildcardRespFile8, _, wildcardContentFile8, _, errFile8 := g.GetRequest(urlFile8)
	if errFile8 != nil {
		return errFile8
	}
	cleanWildcardContentFile8 := strings.ReplaceAll(*wildcardContentFile8, urlFile8, "")
	rsFile8 := r.FindStringSubmatch(*wildcardContentFile8)
	cleanTitleFile8 := ""
	if len(rsFile8) > 0 {
		cleanTitleFile8 = strings.TrimSpace(rsFile8[1])
	}

	if *wildcardRespFile16 == *wildcardRespFile8 {
		g.WildcardStatusCode = wildcardRespFile16
		log.Printf("[-] Wildcard response found: %s => %d", urlFile16, *wildcardRespFile16)
		log.Printf("[-] Wildcard response found: %s => %d", urlFile8, *wildcardRespFile8)
		if cleanTitleFile16 != "" && cleanTitleFile16 == cleanTitleFile8 {
			g.IsWildcardFileByTitle = true
			g.WildcardFileTitle = cleanTitleFile16
			log.Printf(" --> Wildcard by title: %s", cleanTitleFile16)
		} else if len(cleanWildcardContentFile16) == len(cleanWildcardContentFile8) {
			g.IsWildcardFileByContentLength = true
			g.WildcardFileContentLength = len(cleanWildcardContentFile16)
			log.Printf(" --> Wildcard by content length: %d", len(cleanWildcardContentFile16))
		}
	} else {
		log.Printf("[-] Wildcard response NOT found: %s => %d", urlFile16, *wildcardRespFile16)
		log.Printf("[-] Wildcard response NOT found: %s => %d", urlFile8, *wildcardRespFile8)
	}

	uuidDir16 := fmt.Sprintf("%s%s", strings.ReplaceAll(uuid.New().String(), "-", "")[0:15], "/")
	urlDir16 := fmt.Sprintf("%s%s", g.Opts.URL, uuidDir16)
	wildcardRespDir16, _, wildcardContentDir16, _, errDir16 := g.GetRequest(urlDir16)
	if errDir16 != nil {
		return errDir16
	}
	cleanWildcardContentDir16 := strings.ReplaceAll(*wildcardContentDir16, urlDir16, "")
	rsDir16 := r.FindStringSubmatch(*wildcardContentDir16)
	cleanTitleDir16 := ""
	if len(rsDir16) > 0 {
		cleanTitleDir16 = strings.TrimSpace(rsDir16[1])
	}

	uuidDir8 := fmt.Sprintf("%s%s", strings.ReplaceAll(uuid.New().String(), "-", "")[0:7], "/")
	urlDir8 := fmt.Sprintf("%s%s", g.Opts.URL, uuidDir8)
	wildcardRespDir8, _, wildcardContentDir8, _, errDir8 := g.GetRequest(urlDir8)
	if errDir8 != nil {
		return errDir8
	}
	cleanWildcardContentDir8 := strings.ReplaceAll(*wildcardContentDir8, urlDir8, "")
	rsDir8 := r.FindStringSubmatch(*wildcardContentDir8)
	cleanTitleDir8 := ""
	if len(rsDir8) > 0 {
		cleanTitleDir8 = strings.TrimSpace(rsDir8[1])
	}

	if *wildcardRespDir16 == *wildcardRespDir8 {
		g.WildcardStatusCode = wildcardRespDir16
		log.Printf("[-] Wildcard response found: %s => %d", urlDir16, *wildcardRespDir16)
		log.Printf("[-] Wildcard response found: %s => %d", urlDir8, *wildcardRespDir8)
		if cleanTitleDir16 != "" && cleanTitleDir16 == cleanTitleDir8 {
			g.IsWildcardDirByTitle = true
			g.WildcardDirTitle = cleanTitleDir16
			log.Printf(" --> Wildcard by title: %s", cleanTitleDir16)
		} else if len(cleanWildcardContentDir16) == len(cleanWildcardContentDir8) {
			g.IsWildcardDirByContentLength = true
			g.WildcardDirContentLength = len(cleanWildcardContentDir16)
			log.Printf(" --> Wildcard by content length: %d", len(cleanWildcardContentDir16))
		}
	} else {
		log.Printf("[-] Wildcard response NOT found: %s => %d", urlDir16, *wildcardRespDir16)
		log.Printf("[-] Wildcard response NOT found: %s => %d", urlDir8, *wildcardRespDir8)
	}

	return nil
}

// Process is the process implementation of gobusterdir
func (d GobusterDir) Process(g *libgobuster.Gobuster, busterTarget *libgobuster.BusterTarget) ([]libgobuster.Result, error) {
	suffix := ""
	if g.Opts.UseSlash {
		suffix = "/"
	}

	entity := busterTarget.Target
	isEntityURL := true
	url := entity
	var ret []libgobuster.Result

	if !busterTarget.IsURL {
		word := strings.TrimPrefix(busterTarget.Target, "/")
		entity = fmt.Sprintf("%s%s", word, suffix)
		isEntityURL = false
		url = fmt.Sprintf("%s%s", g.Opts.URL, entity)
	}

	if len(g.Opts.RandomAgentParsed) > 0 {
		rand.Seed(time.Now().UTC().UnixNano())
		randomAgent := g.Opts.RandomAgentParsed[rand.Intn(len(g.Opts.RandomAgentParsed))]
		g.HTTP.UserAgent = randomAgent
	}

	dirResp, dirSize, dirContent, redirectURL, err := g.GetRequest(url)
	if err != nil {
		return nil, err
	}

	if dirResp != nil {
		ret = append(ret, libgobuster.Result{
			Entity:      entity,
			Status:      *dirResp,
			Size:        dirSize,
			Content:     dirContent,
			IsEntityURL: isEntityURL,
			RedirectURL: redirectURL,
		})
	}

	return ret, nil
}

// ResultToString is the to string implementation of gobusterdir
func (d GobusterDir) ResultToString(g *libgobuster.Gobuster, r *libgobuster.Result) (*string, *string, int, error) {
	buf := &bytes.Buffer{}
	allBuf := &bytes.Buffer{}
	isFalsePositive := false
	isDir := strings.HasSuffix(r.Entity, "/")
	rgx := regexp.MustCompile(`(?s).*<title>(?P<Title>.*)<\/title>.*`)

	if r.Status == *g.WildcardStatusCode {
		if isDir {
			if g.IsWildcardDirByTitle {
				rsDir := rgx.FindStringSubmatch(*r.Content)
				cleanTitleDir := ""
				if len(rsDir) > 0 {
					cleanTitleDir = strings.TrimSpace(rsDir[1])
					if cleanTitleDir == g.WildcardDirTitle {
						isFalsePositive = true
					}
				}
			} else if g.IsWildcardDirByContentLength {
				entity := r.Entity
				if !r.IsEntityURL {
					entity = fmt.Sprintf("%s%s", g.Opts.URL, entity)
				}
				cleanWildcardContentDir := strings.ReplaceAll(*r.Content, entity, "")
				if len(cleanWildcardContentDir) == g.WildcardDirContentLength {
					isFalsePositive = true
				}
			}
		} else {
			if g.IsWildcardFileByTitle {
				rsFile := rgx.FindStringSubmatch(*r.Content)
				cleanTitleFile := ""
				if len(rsFile) > 0 {
					cleanTitleFile = strings.TrimSpace(rsFile[1])
					if cleanTitleFile == g.WildcardFileTitle {
						isFalsePositive = true
					}
				}
			} else if g.IsWildcardFileByContentLength {
				entity := r.Entity
				if !r.IsEntityURL {
					entity = fmt.Sprintf("%s%s", g.Opts.URL, entity)
				}
				cleanWildcardContentFile := strings.ReplaceAll(*r.Content, entity, "")
				if len(cleanWildcardContentFile) == g.WildcardFileContentLength {
					isFalsePositive = true
				}
			}
		}
	}

	hasExcludeString := false
	if g.Opts.ExcludeString != "" {
		hasExcludeString = strings.Contains(*r.Content, g.Opts.ExcludeString)
	}

	// Prefix if we're in verbose mode
	if g.Opts.Verbose {
		if isFalsePositive {
			if _, err := fmt.Fprintf(buf, "%-16s", "FALSE POSITIVE"); err != nil {
				return nil, nil, 0, err
			}
		} else if !g.Opts.ExcludedStatusCodesParsed.Contains(r.Status) && !hasExcludeString {
			if _, err := fmt.Fprintf(buf, "%-16s", "FOUND"); err != nil {
				return nil, nil, 0, err
			}
		} else {
			if _, err := fmt.Fprintf(buf, "%-16s", "MISSED"); err != nil {
				return nil, nil, 0, err
			}
		}
	}

	t := time.Now()
	if !g.Opts.ExcludedStatusCodesParsed.Contains(r.Status) && !isFalsePositive && !hasExcludeString || g.Opts.Verbose {
		if _, err := fmt.Fprintf(buf, "[%02d:%02d:%02d]", t.Hour(), t.Minute(), t.Second()); err != nil {
			return nil, nil, 0, err
		}

		if _, err := fmt.Fprintf(buf, "%8d", r.Status); err != nil {
			return nil, nil, 0, err
		}

		if r.Size != nil {
			if _, err := fmt.Fprintf(buf, "%12d B", *r.Size); err != nil {
				return nil, nil, 0, err
			}
		} else {
			if _, err := fmt.Fprintf(buf, "%12d B", 0); err != nil {
				return nil, nil, 0, err
			}
		}

		if _, err := fmt.Fprintf(buf, "     -     "); err != nil {
			return nil, nil, 0, err
		}

		if !r.IsEntityURL {
			if _, err := fmt.Fprintf(buf, "%s", g.Opts.URL); err != nil {
				return nil, nil, 0, err
			}
		}

		if _, err := fmt.Fprintf(buf, "%s", r.Entity); err != nil {
			return nil, nil, 0, err
		}

		if *r.RedirectURL != "" {
			if _, err := fmt.Fprintf(buf, "  ->  "); err != nil {
				return nil, nil, 0, err
			}

			if _, err := fmt.Fprintf(buf, "%s", *r.RedirectURL); err != nil {
				return nil, nil, 0, err
			}
		}

		if _, err := fmt.Fprintf(buf, "\n"); err != nil {
			return nil, nil, 0, err
		}

		if _, err := fmt.Fprintf(allBuf, "[%d-%02d-%02d %02d:%02d:%02d] - ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()); err != nil {
			return nil, nil, 0, err
		}

		allBufEntity := r.Entity
		if r.IsEntityURL {
			u, err := url.Parse(allBufEntity)
			if err == nil {
				schemeHost := fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
				allBufEntity = strings.ReplaceAll(allBufEntity, schemeHost, "")
			}
		}

		if _, err := fmt.Fprintf(allBuf, "/%s - ", allBufEntity); err != nil {
			return nil, nil, 0, err
		}

		if _, err := fmt.Fprintf(allBuf, "%d", r.Status); err != nil {
			return nil, nil, 0, err
		}

		if *r.RedirectURL != "" {
			if _, err := fmt.Fprintf(allBuf, "  ->  "); err != nil {
				return nil, nil, 0, err
			}

			if _, err := fmt.Fprintf(allBuf, "%s", *r.RedirectURL); err != nil {
				return nil, nil, 0, err
			}
		}

		if _, err := fmt.Fprintf(allBuf, "\n"); err != nil {
			return nil, nil, 0, err
		}
	}
	s := buf.String()
	as := allBuf.String()
	return &s, &as, r.Status, nil
}
