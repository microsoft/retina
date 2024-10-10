package kubernetes

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultRetinaPort = 10093
	defaultSpanTime   = 10 * time.Second
)

var (

	// key:profile name, value: pprof endpoint
	simpleProfiles = map[string]string{
		"heap":  "/heap",
		"block": "/block",
		"mutex": "/mutex",
	}

	durationProfiles = map[string]string{
		"cpu":       "/profile",
		"trace":     "/trace",
		"goroutine": "/goroutine",
	}
)

type PullPProf struct {
	LocalPort             string
	DurationSeconds       string // full duration which includes as many intervals as possible
	ScrapeIntervalSeconds string // will pull all profiles at this interval

	scraper *PprofScraper
}

func (p *PullPProf) Run() error {
	host := "localhost"
	var err error
	p.scraper, err = NewPprofScraper(host, defaultRetinaPort)
	if err != nil {
		return err
	}

	duration, err := strconv.Atoi(p.DurationSeconds)
	if err != nil {
		return fmt.Errorf("error converting pprof duration to int: %w", err)
	}
	interval, err := strconv.Atoi(p.ScrapeIntervalSeconds)
	if err != nil {
		return fmt.Errorf("error converting pprof interval to int: %w", err)
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancel()

	defer func() {
		if err != nil {
			log.Printf("--pprof viewing commands:--\n")
			log.Printf("heap: \tgo tool pprof -http=:8081 %s\n", "heap.out")
			log.Printf("cpu: \tgo tool pprof -http=:8082 %s\n", "cpu.out")
			log.Printf("block: \tgo tool pprof -http=:8083 %s\n", "block.out")
			log.Printf("mutex: \tgo tool pprof -http=:8084 %s\n", "mutex.out")
			log.Printf("trace: \tgo tool trace -http=:8085 %s\n", "trace.out")
		}
	}()

	scrape := func() error {
		log.Printf("-- scraping pprof profiles --\n")
		folder := "./pprof/" + time.Now().Format("2006.01.02-15:04:05") + "/"
		err = os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			return fmt.Errorf("error creating pprof folder: %w", err)
		}

		for name, path := range simpleProfiles {
			file := folder + name + ".out"
			err = p.scraper.GetProfile(name, path, file)
			if err != nil {
				// don't return here because some data is better than no data,
				// and other profiles might be functional
				log.Printf("error getting %s profile: %v\n", name, err)
			}
		}

		for name, path := range durationProfiles {
			file := folder + name + ".out"
			err = p.scraper.GetProfileWithDuration(name, path, file, defaultSpanTime)
			if err != nil {
				// don't return here because some data is better than no data,
				// and other profiles might be functional
				log.Printf("error getting %s profile: %v\n", name, err)
			}
		}

		log.Printf("-- scraped profiles saved to to %s --\n", folder)
		return err
	}

	// pull initial scrape
	err = scrape()
	if err != nil {
		return fmt.Errorf("error pulling pprof profiles: %w", err)
	}

	// start scraping on intervals
	for {
		select {
		case <-ctx.Done():
			if err != nil {
				return fmt.Errorf("error pulling pprof profiles: %w", err)
			}
			return nil
		case <-ticker.C:
			err = scrape()
		}
	}
}

func (p *PullPProf) Prevalidate() error {
	return nil
}

func (p *PullPProf) Stop() error {
	return nil
}

type PprofScraper struct {
	baseURL *url.URL
}

func NewPprofScraper(host string, port int) (*PprofScraper, error) {
	scraper := &PprofScraper{}
	var err error
	baseURL := fmt.Sprintf("http://%s:%d/debug/pprof", host, port)
	scraper.baseURL, err = url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	return scraper, nil
}

func (p *PprofScraper) GetProfileWithDuration(name, path, outfile string, duration time.Duration) error {
	seconds := int(duration.Seconds())
	log.Printf("getting %s profile for %d seconds...\n", name, seconds)
	profileURL := p.formatURLWithSeconds(seconds)
	profileURL.Path += path
	return p.scrape(profileURL.String(), defaultTimeout+duration, outfile)
}

func (p *PprofScraper) GetProfile(name, path, outfile string) error {
	log.Printf("getting %s profile...\n", name)
	return p.scrape(p.baseURL.String()+path, defaultTimeout, outfile)
}

func (p *PprofScraper) formatURLWithSeconds(seconds int) url.URL {
	// Add query parameters
	queryURL := *p.baseURL
	q := queryURL.Query()
	q.Set("seconds", strconv.Itoa(seconds))
	queryURL.RawQuery = q.Encode()
	return queryURL
}

func (p *PprofScraper) scrape(scrapingURL string, timeout time.Duration, outfile string) error {
	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, scrapingURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error scraping: %w", err)
	}
	defer resp.Body.Close()

	file, err := os.Create(outfile)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying scrape response to file: %w", err)
	}

	return nil
}
