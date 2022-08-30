package dpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dooferlad/jat/utils"

	"github.com/dooferlad/jat/shell"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func Update(args []string) error {
	dir, err := ioutil.TempDir("", "jat-dpkg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	manualPackages := viper.GetStringMap("manual_packages")

	wg := sync.WaitGroup{}

	for name, info := range manualPackages {
		if len(args) == 0 || utils.InList(name, args) {
			wg.Add(1)
			go func(dir, name string, info interface{}) {
				if err := checkAndUpdatePackage(dir, name, info); err != nil {
					logrus.Errorf("while downloading %s: %s", name, err)
				}
				wg.Done()
			}(dir, name, info)
		}
	}

	wg.Wait()

	match := filepath.Join(dir, "*.deb")
	if matches, err := filepath.Glob(match); err == nil && matches != nil {
		shell.Sudo("dpkg", append([]string{"-i"}, matches...)...)

		// TODO: record the just installed version against the version we thought
		// that we downloaded so packages where the URL doesn't map to the version
		// can be tracked. So far this will still need to use something unique
		// from the URL, but realistically a checksum (published, etag) could work.
	}

	return nil
}

func checkAndUpdatePackage(dir, name string, info interface{}) error {
	logrus.Infof("checkAndUpdate a deb %s", name)
	var url, packageName, selector, regexstring, fixedDownloadURL, githubRepo string
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
			fixedDownloadURL = v.(string)
		case "github":
			githubRepo = v.(string)
		default:
			continue
		}
	}

	if packageName == "" {
		packageName = name
	}

	dpkgStatus, err := Query(packageName)
	if dpkgStatus.Status != "install ok installed" {
		logrus.Infof("Not installed: %s, %v", name, dpkgStatus)
		return nil
	}
	if err != nil {
		return err
	}

	if githubRepo == "" {
		if url == "" || packageName == "" || selector == "" && regexstring == "" {
			fmt.Println("Error: Incomplete configuration for", name)
		}
	}

	var downloadURL string
	var sigURL string

	if githubRepo != "" {
		client, err := utils.GithubClient()
		if err != nil {
			return err
		}
		bits := strings.Split(githubRepo, "/")

		release, _, err := client.Repositories.GetLatestRelease(context.Background(), bits[0], bits[1])
		if err != nil {
			return err
		}

		if release.TagName != nil {
			if !utils.VersionMatch(*release.TagName, dpkgStatus.Version) {
				fmt.Printf("%s needs updating: %s (local: %s, remote: %s)\n", name, downloadURL, dpkgStatus.Version, *release.TagName)
				for _, a := range release.Assets {
					if a.Name != nil {
						fileName := *a.Name

						if fileName[len(fileName)-4:] == ".deb" {
							downloadURL = *a.BrowserDownloadURL
						}

						if fileName[len(fileName)-8:] == ".deb.asc" {
							sigURL = *a.BrowserDownloadURL
						}
					}
				}
			} else {
				fmt.Printf("%s is up to date (%s)\n", name, dpkgStatus.Version)
				return nil
			}
		}
	}

	if downloadURL == "" {
		downloadURL, err = utils.DownloadFromURL(url, selector, regexstring, dpkgStatus.Version, name, fixedDownloadURL)
		if err != nil {
			return err
		}
		if downloadURL == "" {
			return nil
		}
	}

	// TODO clearly this is the only bit that should be different for binary / dpkg / etc

	fileName := filepath.Join(dir, name+".deb")
	err = utils.DownloadFile(downloadURL, fileName)
	if err != nil {
		return err
	}

	if sigURL != "" {
		sigFile := filepath.Join(dir, name+".deb.asc")
		err := utils.DownloadFile(sigURL, sigFile)
		if err != nil {
			return err
		}

		err = shell.Shell("gpg2", "--verify", sigFile, fileName)
		if err != nil {
			os.Remove(fileName)
			return err
		}
	}

	fmt.Printf("Downloaded new package: %s\n", fileName)

	return nil
}

type Version struct {
	Version string
	Status  string
}

func Query(packageName string) (*Version, error) {
	dpkg := Version{}
	exe := exec.Command("/usr/bin/dpkg-query", "--showformat={\"version\":\"${Version}\",\"status\":\"${Status}\"}", "--show", packageName)
	out, err := exe.CombinedOutput()
	if err != nil {
		return &dpkg, err
	}

	logrus.Infof("%s", out)

	if err := json.Unmarshal(out, &dpkg); err != nil {
		return &dpkg, err
	}

	return &dpkg, nil
}
