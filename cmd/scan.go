package cmd

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
  "strings"
  "time"
  "crypto/tls"

  "github.com/spf13/cobra"
  "github.com/cheggaaa/pb/v3"
)

var domains string
var payloads string
var userAgent string
var timeout int
var threads int
var delay int
var verbose bool
var version = "v0.0.1"

func crlfMapCmd() *cobra.Command {
  crlfMapCmd := &cobra.Command {
    Use:   "scan",
    Short: "A scanner for all your CRLF needs",
    Run: crlfMapFunc,
  }

  crlfMapCmd.Flags().StringVarP(&domains, "domains", "d", "", "Location of domains with parameters to scan")
  crlfMapCmd.Flags().StringVarP(&payloads, "payloads", "p", "payloads.txt", "Location of payloads to generate on requests")
  crlfMapCmd.Flags().StringVarP(&userAgent, "user-agent", "u", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36", "User agent for requests")
  crlfMapCmd.Flags().IntVarP(&timeout, "timeout", "", 10, "The amount of time needed to close a connection that could be hung")
  crlfMapCmd.Flags().IntVarP(&delay, "delay", "", 0, "The time each threads waits between requests in milliseconds")
  crlfMapCmd.Flags().IntVarP(&threads, "threads", "t", 1, "Number of threads to run crlfmap on")
  crlfMapCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

  crlfMapCmd.MarkFlagRequired("domains")

  return crlfMapCmd
}

func crlfMapFunc(cmd *cobra.Command, args []string) {
    var wg sync.WaitGroup

    fmt.Printf(`
           _  __                       
          | |/ _|                      
  ___ _ __| | |_ _ __ ___   __ _ _ __  
 / __| '__| |  _| '_ ' _  \/ _' | '_ \ 
| (__| |  | | | | | | | | | (_| | |_) |
 \___|_|  |_|_| |_| |_| |_|\__,_| .__/ 
                                | |    
                                |_|    

    %s                                
-----------------------
:: Domains    : %s
:: Payloads   : %s
:: Threads    : %d
:: User Agent : %s
:: Timeout    : %d
:: Delay      : %d
-----------------------
`, version, domains, payloads, threads, userAgent, timeout, delay)

    if threads <= 0 {
      fmt.Println("Threads must be larger than 0")
      os.Exit(1)
    }

    payloadsFile := fileReader(payloads)
    domainsFile := fileReader(domains)
    progressBar := pb.New(len(domainsFile))

    for _, domain := range domainsFile {
      for _, payload := range payloadsFile {

        fuzzedURL := fuzzURL(domain, payload)

        for ithreads := 0; ithreads < threads; ithreads++ {
          for _, requestURI := range *fuzzedURL {
            if verbose == false {
              progressBar.Start()
            }
            wg.Add(1)
            progressBar.Increment()
            go makeRequest(requestURI, timeout, &wg)
            if delay > 0 {
              time.Sleep(time.Duration(delay) * time.Millisecond)
            }
            wg.Wait()
          }
        }
        wg.Wait()
    }
  }
  progressBar.Finish()
}

func fuzzURL(domain string, payload string) *[]string {
	var fuzzedURL []string
  var fuzzedParams []string

  // Make sure parameter are present
  if strings.Contains(domain, "?") {
    paramStr := strings.Split(domain, "?")[1]
    params := strings.Split(paramStr, "&")
    domainPrefix := strings.Split(domain, "?")[0]
    URL := domainPrefix+"?"

    paramFuzzCount := 0
    // Rebuild parameters so we can work with each parameter individually (I may be doing this wrong)
    // Clear list before concatentation again
    fuzzedParams = nil
    for _, param := range params {
      fuzzedParams = append(fuzzedParams,param)

      if paramFuzzCount != (len(params) - 1) {
        fuzzedParams = append(fuzzedParams,"&")
      }
      paramFuzzCount += 1
    }

    // Inject payload into each parameter consecutively.  We don't want to 
    // have server errors for actions that could require specific strings
    for paramPayloadCount := 0; paramPayloadCount < len(fuzzedParams); paramPayloadCount++ {
      finalFuzzedParams := make([]string, len(fuzzedParams))
      copy(finalFuzzedParams, fuzzedParams)
      finalFuzzedParams[paramPayloadCount] = fuzzedParams[paramPayloadCount] + payload

      flattenedURL := URL+strings.Join(finalFuzzedParams[:], "")
      fuzzedURL = append(fuzzedURL,flattenedURL)
    }
  }
  return &fuzzedURL
}

func fileReader(ulist string) []string {
	var buffer []string
	file, err := os.Open(ulist)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		list := scanner.Text()
		buffer = append(buffer, list)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return buffer

}

func makeRequest(uri string, timeoutFlag int, wg *sync.WaitGroup) {
  defer wg.Done()

	URL := uri

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
    },
    Timeout: time.Duration(timeoutFlag)*time.Second,
    Transport: &http.Transport{
      MaxIdleConns: 100,
      MaxIdleConnsPerHost: 100,
      TLSClientConfig: &tls.Config{
        InsecureSkipVerify: true,
      },
    }}

	resp, err := client.Get(URL)

	if err != nil {
    if verbose == true {
      fmt.Println(err)
    }
		return
	}


  if verbose == true {
    fmt.Printf("%s (Status : %d)\n", URL, resp.StatusCode)
  }

	for key := range resp.Header {
		if key == "Injected-Header" {
			fmt.Println("[+]" + URL + ": is Vulnerable")
		}

	}
}

func init() {
  rootCmd.AddCommand(crlfMapCmd())
}