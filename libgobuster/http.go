package libgobuster

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

type httpClient struct {
	client        *http.Client
	context       context.Context
	UserAgent     string
	username      string
	password      string
	includeLength bool
}

// NewHTTPClient returns a new HTTPClient
func newHTTPClient(c context.Context, opt *Options) (*httpClient, error) {
	var proxyURLFunc func(*http.Request) (*url.URL, error)
	var client httpClient
	proxyURLFunc = http.ProxyFromEnvironment

	if opt == nil {
		return nil, fmt.Errorf("options is nil")
	}

	if opt.Proxy != "" {
		proxyURL, err := url.Parse(opt.Proxy)
		if err != nil {
			return nil, fmt.Errorf("proxy URL is invalid (%v)", err)
		}
		proxyURLFunc = http.ProxyURL(proxyURL)
	}

	var redirectFunc func(req *http.Request, via []*http.Request) error
	if !opt.FollowRedirect {
		redirectFunc = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		redirectFunc = nil
	}

	client.client = &http.Client{
		Timeout:       opt.Timeout,
		CheckRedirect: redirectFunc,
		Transport: &http.Transport{
			Proxy: proxyURLFunc,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opt.InsecureSSL,
			},
		}}
	client.context = c
	client.username = opt.Username
	client.password = opt.Password
	client.includeLength = opt.IncludeLength
	client.UserAgent = opt.UserAgent
	return &client, nil
}

// MakeRequest makes a request to the specified url
func (client *httpClient) makeRequest(fullURL, cookie string) (*int, *int64, *string, *string, error) {
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)

	if err != nil {
		return nil, nil, nil, nil, err
	}

	// add the context so we can easily cancel out
	req = req.WithContext(client.context)

	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	ua := fmt.Sprintf("gobuster %s", VERSION)
	if client.UserAgent != "" {
		ua = client.UserAgent
	}
	req.Header.Set("User-Agent", ua)

	if client.username != "" {
		req.SetBasicAuth(client.username, client.password)
	}

	resp, err := client.client.Do(req)
	if err != nil {
		if ue, ok := err.(*url.Error); ok {

			if strings.HasPrefix(ue.Err.Error(), "x509") {
				return nil, nil, nil, nil, fmt.Errorf("Invalid certificate: %v", ue.Err)
			}
		}
		return nil, nil, nil, nil, err
	}

	defer resp.Body.Close()

	var length *int64
	length = new(int64)
	var content *string
	content = new(string)

	body, err2 := ioutil.ReadAll(resp.Body)
	if err2 == nil {
		*content = string(body)
		*length = int64(utf8.RuneCountInString(*content))
	}

	if client.includeLength {
		if resp.ContentLength > 0 {
			*length = resp.ContentLength
		}
	} else {
		// DO NOT REMOVE!
		// absolutely needed so golang will reuse connections!
		_, err = io.Copy(ioutil.Discard, resp.Body)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	var redirectURL *string
	redirectURL = new(string)
	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		value, err := resp.Location()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		*redirectURL = value.String()
	} else {
		*redirectURL = ""
	}

	return &resp.StatusCode, length, content, redirectURL, nil
}
