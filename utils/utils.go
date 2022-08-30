package utils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/github"
	"github.com/spf13/viper"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

type VersionError struct {
	Package string
}

func (e VersionError) Error() string {
	return fmt.Sprintf("Unable to determine remote version for %v", e.Package)
}

type withHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithHeader(rt http.RoundTripper) withHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return withHeader{Header: make(http.Header), rt: rt}
}

func (h withHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.Header {
		req.Header[k] = v
	}

	return h.rt.RoundTrip(req)
}

func GithubClient() (*github.Client, error) {
	authSettings := viper.GetStringMap("auth")
	githubAuth, ok := authSettings["github"]
	var username, token string
	if ok {
		for k, v := range githubAuth.(map[string]interface{}) {
			switch k {
			case "username":
				username = v.(string)
			case "token":
				token = v.(string)
			default:
				continue
			}
		}
	}

	httpClient := http.DefaultClient

	if username != "" && token != "" {
		rt := WithHeader(httpClient.Transport)
		rt.Set("Authorization", username+":"+token)
		httpClient.Transport = rt
	} else if username != "" || token != "" {
		return nil, errors.New("Both username and token needed for github auth")
	}

	client := github.NewClient(httpClient)
	return client, nil
}

func DownloadFile(downloadURL, fileName string) error {

	tmp, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer tmp.Close()

	downloadResp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer downloadResp.Body.Close()

	_, err = io.Copy(tmp, downloadResp.Body)
	if err != nil {
		return err
	}
	return nil
}

func VersionFromURL(url string, selector string, regexstring string, name string, downloadURL string) (string, string, error) {
	var remoteVersion string
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if selector != "" {
		doc, err := html.Parse(resp.Body)
		if err != nil {
			return "", "", err
		}

		s, err := cascadia.Compile(selector)
		if err != nil {
			return "", "", err
		}

		node := s.MatchFirst(doc)
		if node == nil {
			return "", "", fmt.Errorf("unable to find download on page %s with selector %s", url, selector)
		}
		fmt.Println(node.FirstChild.Data)

		re, err := regexp.Compile(regexstring)
		if err != nil {
			return "", "", err
		}
		matches := re.FindSubmatch([]byte(node.FirstChild.Data))
		if matches == nil {
			return "", "", fmt.Errorf("unable to find download on page %s with selector %s", url, selector)
		}

		var webVersionFragments []string
		for _, m := range matches[1:] {
			webVersionFragments = append(webVersionFragments, string(m))
		}

		remoteVersion = strings.Join(webVersionFragments, ".")
	} else if regexstring != "" {
		downloadURL = "-"

		doc, err := html.Parse(resp.Body)
		if err != nil {
			return "", "", err
		}

		re, err := regexp.Compile(regexstring)
		if err != nil {
			return "", "", err
		}

		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						// fmt.Printf("  %s\n", a.Val)
						matches := re.FindSubmatch([]byte(a.Val))
						if len(matches) > 0 && len(matches[1]) > 0 {
							downloadURL = a.Val
							remoteVersion = string(matches[1])
							return
						}
						break
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
			return
		}
		f(doc)
	}

	if remoteVersion == "" {
		return "", "", VersionError{name}
	}

	return remoteVersion, downloadURL, nil
}

func DownloadFromURL(url string, selector string, regexstring string, version string, name string, downloadURL string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if selector != "" {
		doc, err := html.Parse(resp.Body)
		if err != nil {
			return "", err
		}

		s, err := cascadia.Compile(selector)
		if err != nil {
			return "", err
		}

		node := s.MatchFirst(doc)
		if node == nil {
			return "", fmt.Errorf("unable to find download on page %s with selector %s", url, selector)
		}
		fmt.Println(node.FirstChild.Data)

		re, err := regexp.Compile(regexstring)
		if err != nil {
			return "", err
		}
		matches := re.FindSubmatch([]byte(node.FirstChild.Data))
		if matches == nil {
			return "", fmt.Errorf("unable to find download on page %s with selector %s", url, selector)
		}

		var webVersionFragments []string
		for _, m := range matches[1:] {
			webVersionFragments = append(webVersionFragments, string(m))
		}

		remoteVersion := strings.Join(webVersionFragments, ".")
		if !VersionMatch(remoteVersion, version) {
			fmt.Printf("%s needs updating: %s (local: %s, remote: %s)\n", name, downloadURL, version, remoteVersion)
		} else {
			fmt.Printf("%s is up to date (%s)\n", name, version)
			return "", nil
		}
	} else if regexstring != "" {
		downloadURL = "-"

		doc, err := html.Parse(resp.Body)
		if err != nil {
			return "", err
		}

		re, err := regexp.Compile(regexstring)
		if err != nil {
			return "", err
		}

		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						fmt.Printf("  %s\n", a.Val)
						matches := re.FindSubmatch([]byte(a.Val))
						if len(matches) > 0 && len(matches[1]) > 0 {
							remoteVersion := string(matches[1])
							if !VersionMatch(remoteVersion, version) {
								downloadURL = a.Val
								fmt.Printf("%s needs updating: %s (local: %s, remote: %s)\n", name, downloadURL, version, remoteVersion)
							} else {
								fmt.Printf("%s is up to date (%s)\n", name, version)
								downloadURL = ""
							}
							return
						}
						break
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc)

		if downloadURL == "" {
			return "", nil
		}

		if downloadURL == "-" {
			return "", VersionError{name}
		}
	}
	return downloadURL, nil
}

func VersionMatch(remote, local string) bool {
	remote = strings.TrimLeft(remote, "vV")
	local = strings.TrimLeft(local, "vV")

	dashIndex := strings.Index(local, "-")
	if dashIndex >= 0 {
		local = local[:dashIndex]
	}

	r := strings.Split(remote, ".")
	l := strings.Split(local, ".")

	for i, v := range r {
		if i >= len(l) {
			break
		}

		if v != l[i] {
			return false
		}
	}

	return true
}

func InList(s string, list []string) bool {
	for _, a := range list {
		if a == s {
			return true
		}
	}
	return false
}
