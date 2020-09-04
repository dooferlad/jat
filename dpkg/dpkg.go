package dpkg

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/dooferlad/jat/shell"
	"github.com/spf13/viper"
	"golang.org/x/net/html"
)

func Update() error {
	manualPackages := viper.GetStringMap("manual_packages")
	for name, info := range manualPackages {
		if err := checkAndUpdatePackage(name, info); err != nil {
			return err
		}
	}

	return nil
}

func checkAndUpdatePackage(name string, info interface{}) error {
	var url, packageName, selector, regexstring, downloadURL string
	infoMap := info.(map[string]interface{})
	for k, v := range infoMap {
		switch k {
		case "url":
			url = v.(string)
		case "name":
			packageName = v.(string)
		case "selector":
			selector = v.(string)
		case "regexp":
			regexstring = v.(string)
		case "download":
			downloadURL = v.(string)
		default:
			continue
		}
	}

	if url == "" || packageName == "" || selector == "" && regexstring == "" {
		fmt.Println("Error: Incomplete configuration for ", name)
	}

	exe := exec.Command("/usr/bin/dpkg-query", "--showformat='${Version}'", "--show", packageName)
	out, err := exe.CombinedOutput()
	if err != nil {
		return err
	}

	var version string
	version = string(out[1 : len(out)-1])

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if selector != "" {
		doc, err := html.Parse(resp.Body)
		if err != nil {
			return err
		}

		s, err := cascadia.Compile(selector)
		if err != nil {
			return err
		}

		node := s.MatchFirst(doc)
		fmt.Println(node.FirstChild.Data)

		re, err := regexp.Compile(regexstring)
		if err != nil {
			return err
		}
		matches := re.FindSubmatch([]byte(node.FirstChild.Data))
		if !versionMatch(string(matches[1]), version) {
			fmt.Printf("%s needs updating: %s\n", name, downloadURL)
		} else {
			fmt.Printf("%s is up to date (%s)\n", name, version)
			return nil
		}
	} else if regexstring != "" {
		doc, err := html.Parse(resp.Body)
		if err != nil {
			return err
		}

		re, err := regexp.Compile(regexstring)
		if err != nil {
			return err
		}

		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						matches := re.FindSubmatch([]byte(a.Val))
						if len(matches) > 0 && len(matches[1]) > 0 {
							if !versionMatch(string(matches[1]), version) {
								downloadURL = a.Val
								fmt.Printf("%s needs updating: %s\n", name, downloadURL)
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
			return nil
		}
	}

	dir, err := ioutil.TempDir("", "jat")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	fileName := filepath.Join(dir, name+".deb")
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

	shell.Sudo("dpkg", "-i", fileName)

	return nil
}

func versionMatch(remote, local string) bool {
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
