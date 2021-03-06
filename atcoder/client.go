package atcoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/gocolly/colly"
)

type Sample struct {
	Input  string
	Output string
}

type Client struct {
	baseURL   string
	collector *colly.Collector

	useCache     bool
	cacheDirPath string

	outStream io.Writer
	errStream io.Writer
}

func NewClient(baseURL string, useCache bool, cacheDirPath string, outStream, errStream io.Writer) *Client {
	return &Client{
		baseURL:      baseURL,
		collector:    colly.NewCollector(),
		useCache:     useCache,
		cacheDirPath: cacheDirPath,
		outStream:    outStream,
		errStream:    errStream,
	}
}

func (c *Client) IsContestBeingHeld(contestURL string) (bool, error) {
	beingHeld := false
	c.collector.OnHTML(`form > button.btn-lg.center-block`, func(e *colly.HTMLElement) {
		beingHeld = true
	})

	if err := c.collector.Visit(contestURL); err != nil {
		return false, fmt.Errorf("could not get HTML: %s", contestURL)
	}

	return beingHeld, nil
}

func (c *Client) LogIn(username, password string) error {
	if username == "" || password == "" {
		return errors.New("you need to provide username and password as command line options to test for the contest being held")
	}

	var (
		csrfToken string
		loginErr  error
	)
	loginURL := c.baseURL + "/login"

	c.collector.OnHTML(`input[name="csrf_token"]`, func(e *colly.HTMLElement) {
		if csrfToken != "" {
			return
		}

		csrfToken, _ = e.DOM.Attr("value")
		reqBody := map[string]string{
			"username":   username,
			"password":   password,
			"csrf_token": csrfToken,
		}

		if err := c.collector.Post(loginURL, reqBody); err != nil {
			loginErr = fmt.Errorf("login error: %s", err)
			return
		}
		if !c.isLoggedIn(username) {
			loginErr = fmt.Errorf("login error: username/password may be wrong")
			return
		}
	})

	if err := c.collector.Visit(loginURL); err != nil {
		return fmt.Errorf("could not get HTML: %s", loginURL)
	}

	return loginErr
}

func (c *Client) GetProblemURL(contest, problem string) (string, error) {
	var problemURL string
	c.collector.OnHTML(`td > a[href]`, func(e *colly.HTMLElement) {
		e.DOM.First()
		if e.Text == strings.ToUpper(problem) {
			problemURL = c.baseURL + e.Attr("href")
		}
	})

	problemListURL := fmt.Sprintf("%s/contests/%s/tasks", c.baseURL, strings.ToLower(contest))
	if err := c.collector.Visit(problemListURL); err != nil {
		return "", fmt.Errorf("could not get HTML: %s", problemListURL)
	}

	if problemURL == "" {
		return "", fmt.Errorf("could not find problem page for problem '%s' of contest '%s'", problem, contest)
	}
	return problemURL, nil
}

func (c *Client) GetSamples(problemURL string) ([]Sample, error) {
	cacheFilePath := c.cacheFilePath(problemURL)
	if c.useCache {
		if samples, ok := c.getCachedSamples(cacheFilePath); ok {
			return samples, nil
		}
	}

	elements, err := c.fetchSampleElements(problemURL)
	if err != nil {
		return nil, err
	}

	samples, err := c.constructSamples(elements)
	if err != nil {
		return nil, err
	}

	if err := c.cacheSamples(cacheFilePath, samples); err != nil {
		_, _ = io.WriteString(c.errStream, err.Error())
	}

	return samples, nil
}

func (c *Client) isLoggedIn(username string) bool {
	for _, c := range c.collector.Cookies(c.baseURL) {
		if strings.Contains(c.Value, "UserScreenName%3A"+username) {
			return true
		}
	}
	return false
}

func (c *Client) cacheFilePath(problemURL string) string {
	escapedURL := strings.Replace(problemURL, "/", "_", -1)
	filename := fmt.Sprintf("%s.json", escapedURL)
	return path.Join(c.cacheDirPath, filename)
}

func (c *Client) getCachedSamples(cacheFilePath string) ([]Sample, bool) {
	_, err := os.Stat(c.cacheDirPath)
	if err != nil {
		return nil, false
	}

	bytes, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		return nil, false
	}

	var samples []Sample
	if err := json.Unmarshal(bytes, &samples); err != nil {
		return nil, false
	}

	return samples, true
}

func (c *Client) cacheSamples(cacheFilePath string, samples []Sample) error {
	_, err := os.Stat(c.cacheDirPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(c.cacheDirPath, 0777); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	bytes, err := json.Marshal(samples)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cacheFilePath, bytes, 0644)
}

func (c *Client) fetchSampleElements(problemURL string) (map[string]string, error) {
	elements := make(map[string]string)
	c.collector.OnHTML(`pre`, func(e *colly.HTMLElement) {
		title := e.DOM.Parent().Find("h3").Text()
		if strings.HasPrefix(title, "入力例") || strings.HasPrefix(title, "出力例") {
			titleKey := strings.Replace(title, " ", "", -1)
			elements[titleKey] = e.Text
		} else {
			title := e.DOM.Parent().Parent().Find("h3").Text()
			if strings.HasPrefix(title, "入力例") || strings.HasPrefix(title, "出力例") {
				titleKey := strings.Replace(title, " ", "", -1)
				elements[titleKey] = e.Text
			}
		}
	})

	if err := c.collector.Visit(problemURL); err != nil {
		return nil, fmt.Errorf("could not get HTML: %s", problemURL)
	}

	return elements, nil
}

func (c *Client) constructSamples(elements map[string]string) ([]Sample, error) {
	if len(elements) == 0 {
		return nil, errors.New("no sample elements found")
	}
	if len(elements)%2 != 0 {
		return nil, fmt.Errorf("number of sample elements should be even because it consists of pair of input/output. got: %d", len(elements))
	}

	numSamples := len(elements) / 2
	samples := make([]Sample, numSamples)

	// for html which only has one pair without numbering ["入力例", "出力例"] (without numbering)
	if numSamples == 1 {
		if input, ok := elements["入力例"]; ok {
			if output, ok := elements["出力例"]; ok {
				samples[0] = Sample{Input: input, Output: output}
				return samples, nil
			}
		}
	}

	// for html which has pairs of samples with numbering ["入力例 1", "出力例 1", "入力例 2", ...]
	for i := 1; i <= numSamples; i++ {
		inputKey := fmt.Sprintf("入力例%d", i)
		outputKey := fmt.Sprintf("出力例%d", i)

		input, ok := elements[inputKey]
		if !ok {
			return nil, fmt.Errorf("could not find '%s' in HTML", inputKey)
		}
		output, ok := elements[outputKey]
		if !ok {
			return nil, fmt.Errorf("could not find '%s' in HTML", outputKey)
		}

		samples[i-1] = Sample{Input: input, Output: output}
	}

	return samples, nil
}
