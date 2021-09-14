package main

//----------------------------------------------------
// Gobuster -- by OJ Reeves
//
// A crap attempt at building something that resembles
// dirbuster or dirb using Go. The goal was to build
// a tool that would help learn Go and to actually do
// something useful. The idea of having this compile
// to native code is also appealing.
//
// Run: gobuster -h
//
// Please see THANKS file for contributors.
// Please see LICENSE file for license details.
//
//----------------------------------------------------

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"yBuster/gobusterdir"
	"yBuster/gobusterdns"
	"yBuster/libgobuster"

	"github.com/gookit/color"
	"golang.org/x/crypto/ssh/terminal"
)

func ruler() {
	fmt.Println("===============================================================")
}

func banner() {
	fmt.Printf("yBuster v%s              Custom by Y\n", libgobuster.VERSION)
}

func resultWorker(g *libgobuster.Gobuster, filename string, outputfolder string, wg *sync.WaitGroup) {
	defer wg.Done()
	var f *os.File
	var af *os.File
	var err error
	var aerr error
	var aerrz error

	if len(outputfolder) == 0 {
		log.Fatalf("Output folder cannot be null.")

	} else {

		if _, ferrz := os.Stat(outputfolder); os.IsNotExist(ferrz) {
			errDir := os.MkdirAll(outputfolder, 0755)
			if errDir != nil {
				log.Fatalf("error on creating main output folder: %v", aerrz)
			}
		}
		if _, ferrz := os.Stat(outputfolder + "/output_matches/"); os.IsNotExist(ferrz) {
			errDir := os.MkdirAll(outputfolder + "/output_matches/", 0755)
			if errDir != nil {
				log.Fatalf("error on creating matches output folder: %v", aerrz)
			}
		}
		if _, ferrz := os.Stat(outputfolder + "/output_waybackurls/"); os.IsNotExist(ferrz) {
			errDir := os.MkdirAll(outputfolder + "/output_waybackurls/", 0755)
			if errDir != nil {
				log.Fatalf("error on creating waybackurls output folder: %v", aerrz)
			}
		}
	}


	if filename != "" {
		f, err = os.Create(outputfolder + "/" + filename)
		if err != nil {
			log.Fatalf("error on creating output file: %v", err)
		}
	} else {
		filenameTimeStamp := int32(time.Now().Unix())
		parsedMainURL, _ := url.Parse(g.Opts.URL)
		sanitizedHost := strings.ReplaceAll(parsedMainURL.Host, ".", "_")
		sanitizedHost = strings.ReplaceAll(sanitizedHost, ":", "_")
		sanitizedPath := ""
		if parsedMainURL.Path != "/" {
			sanitizedPath = strings.TrimSuffix(parsedMainURL.Path, "/")
			sanitizedPath = strings.ReplaceAll(sanitizedPath, "/", "_")
		}

		autoFilename := fmt.Sprintf(outputfolder + "/output_matches/matches_%d_%s_%s%s.txt", filenameTimeStamp, parsedMainURL.Scheme, sanitizedHost, sanitizedPath)
		f, err = os.Create(autoFilename)
		if err != nil {
			log.Fatalf("error on creating output file: %v", err)
		}
	}


 

	if _, ferr := os.Stat(outputfolder +"/all_time_matches.txt"); os.IsNotExist(ferr) {
		af, aerr = os.Create(outputfolder + "/all_time_matches.txt")
		if aerr != nil {
			log.Fatalf("error on creating all time matches file: %v", aerr)
		}
	} else {
		af, aerr = os.OpenFile(outputfolder + "/all_time_matches.txt", os.O_APPEND|os.O_WRONLY, 0600)
		if aerr != nil {
			log.Fatalf("error on opening all time matches file: %v", aerr)
		}
	}
	defer af.Close()

	for r := range g.Results() {
		s, as, status, err := r.ToString(g)
		if err != nil {
			log.Fatal(err)
		}
		if s != "" {
			g.ClearProgress()
			s = strings.TrimSpace(s)
			c := color.Style{color.White}
			if status == 200 {
				c = color.Style{color.FgGreen, color.OpBold}
			} else if status == 301 || status == 302 {
				c = color.Style{color.FgYellow, color.OpBold}
			} else if status == 400 {
				c = color.Style{color.FgWhite, color.OpBold}
			} else if status == 401 {
				c = color.Style{color.FgCyan, color.OpBold}
			} else if status == 403 {
				c = color.Style{color.FgMagenta, color.OpBold}
			} else if status == 500 {
				c = color.Style{color.FgRed, color.OpBold}
			}
			c.Println(s)
			if f != nil {
				err = writeToFile(f, s)
				if err != nil {
					log.Fatalf("error on writing output file: %v", err)
				}
			}
		}
		if as != "" {
			as = strings.TrimSpace(as)
			if af != nil {
				werr := writeToFile(af, as)
				if werr != nil {
					log.Fatalf("error on writing all time matches file: %v", err)
				}
			}
		}
	}
}

func errorWorker(g *libgobuster.Gobuster, wg *sync.WaitGroup) {
	defer wg.Done()
	for e := range g.Errors() {
		g.IncrementErrorCount()
		g.DecrementRequests()
		if !g.Opts.Quiet {
			g.ClearProgress()
			if g.Opts.Verbose {
				log.Printf("[!] %v", e)
			}
		}
	}
}

func progressWorker(c context.Context, g *libgobuster.Gobuster) {
	tick := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-tick.C:
			g.PrintProgress()
		case <-c.Done():
			return
		}
	}
}

func writeToFile(f *os.File, output string) error {
	_, err := f.WriteString(fmt.Sprintf("%s\n", output))
	if err != nil {
		return fmt.Errorf("[!] Unable to write to file %v", err)
	}
	return nil
}

func main() {
	// var outputFilename string
	o := libgobuster.NewOptions()
	flag.IntVar(&o.Threads, "t", 10, "Number of concurrent threads")
	flag.StringVar(&o.Mode, "m", "dir", "Directory/File mode (dir)")
	flag.StringVar(&o.Wordlist, "w", "", "Path to the wordlist")
	flag.StringVar(&o.OutputFolder, "of", "", "Path to output folder directory")
	flag.StringVar(&o.ExcludedStatusCodes, "x", "", "Excluded status codes (dir mode only)")
	flag.StringVar(&o.OutputFilename, "o", "", "Output file to write results to (defaults to stdout)")
	flag.StringVar(&o.URL, "u", "", "The target URL or Domain")
	flag.StringVar(&o.Cookies, "c", "", "Cookies to use for the requests (dir mode only)")
	flag.StringVar(&o.Username, "U", "", "Username for Basic Auth (dir mode only)")
	flag.StringVar(&o.Password, "P", "", "Password for Basic Auth (dir mode only)")
	flag.StringVar(&o.Extensions, "ext", "", "File extension(s) to search for (dir mode only)")
	flag.StringVar(&o.UserAgent, "a", "", "Set the User-Agent string (dir mode only)")
	flag.StringVar(&o.Proxy, "p", "", "Proxy to use for requests [http(s)://host:port] (dir mode only)")
	flag.DurationVar(&o.Timeout, "to", 10*time.Second, "HTTP Timeout in seconds (dir mode only)")
	flag.BoolVar(&o.Verbose, "v", false, "Verbose output (errors)")
	flag.BoolVar(&o.ShowIPs, "i", false, "Show IP addresses (dns mode only)")
	flag.BoolVar(&o.ShowCNAME, "cn", false, "Show CNAME records (dns mode only, cannot be used with '-i' option)")
	flag.BoolVar(&o.FollowRedirect, "r", false, "Follow redirects")
	flag.BoolVar(&o.Quiet, "q", false, "Don't print the banner and other noise")
	flag.BoolVar(&o.Expanded, "e", false, "Expanded mode, print full URLs")
	flag.BoolVar(&o.NoStatus, "n", false, "Don't print status codes")
	flag.BoolVar(&o.IncludeLength, "l", false, "Include the length of the body in the output (dir mode only)")
	flag.BoolVar(&o.UseSlash, "f", false, "Append a forward-slash to each directory request (dir mode only)")
	flag.BoolVar(&o.WildcardForced, "fw", false, "Force continued operation when wildcard found")
	flag.BoolVar(&o.InsecureSSL, "k", false, "Skip SSL certificate verification")
	flag.BoolVar(&o.NoProgress, "np", false, "Don't display progress")
	flag.StringVar(&o.WaybackUrls, "waybackurls", "", "Path to the wayback urls")
	flag.StringVar(&o.TargetUrls, "targeturls", "", "Path to the target urls")
	flag.StringVar(&o.RandomAgent, "random-agent", "", "Path to the random agent file")
	flag.StringVar(&o.ExcludeString, "xs", "", "Response content string to exclude")
	flag.BoolVar(&o.BlankExtension, "be", false, "Request word without extension")

	flag.Parse()

	// Prompt for PW if not provided
	if o.Username != "" && o.Password == "" {
		fmt.Printf("[?] Auth Password: ")
		passBytes, err := terminal.ReadPassword(int(syscall.Stdin))
		// print a newline to simulate the newline that was entered
		// this means that formatting/printing after doesn't look bad.
		fmt.Println("")
		if err != nil {
			log.Fatal("[!] Auth username given but reading of password failed")
		}
		o.Password = string(passBytes)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var plugin libgobuster.GobusterPlugin
	switch o.Mode {
	case libgobuster.ModeDir:
		plugin = gobusterdir.GobusterDir{}
	case libgobuster.ModeDNS:
		plugin = gobusterdns.GobusterDNS{}
	}

	gobuster, err := libgobuster.NewGobuster(ctx, o, plugin)
	if err != nil {
		log.Fatalf("[!] %v", err)
	}

	if !o.Quiet {
		fmt.Println("")
		ruler()
		banner()
		ruler()
		c, err := gobuster.GetConfigString()
		if err != nil {
			log.Fatalf("error on creating config string: %v", err)
		}
		fmt.Println(c)
		ruler()
		log.Println("Starting yBuster")
		ruler()
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			// caught CTRL+C
			if !gobuster.Opts.Quiet {
				fmt.Println("\n[!] Keyboard interrupt detected, terminating.")
			}
			cancel()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go errorWorker(gobuster, &wg)
	go resultWorker(gobuster, o.OutputFilename, o.OutputFolder, &wg)

	if !o.Quiet && !o.NoProgress {
		go progressWorker(ctx, gobuster)
	}

	if err := gobuster.Start(); err != nil {
		log.Printf("[!] %v", err)
	} else {
		// call cancel func to free ressources and stop progressFunc
		cancel()
		// wait for all output funcs to finish
		wg.Wait()
	}

	if !o.Quiet {
		gobuster.ClearProgress()
		ruler()
		log.Println("Finished")
		ruler()
	}
}
